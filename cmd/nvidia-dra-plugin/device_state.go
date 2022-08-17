/*
 * Copyright (c) 2022, NVIDIA CORPORATION.  All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"fmt"
	"sync"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
	nvcrd "github.com/NVIDIA/k8s-dra-driver/pkg/crd/nvidia/v1/api"
	cdiapi "github.com/container-orchestrated-devices/container-device-interface/pkg/cdi"
)

type UnallocatedDevices []UnallocatedDeviceInfo
type AllocatedDevices map[string]AllocatedDeviceInfo
type ClaimAllocations map[string]AllocatedDevices

type GpuInfo struct {
	uuid       string
	name       string
	minor      int
	migEnabled bool
}

func (g GpuInfo) CDIDevice() string {
	return fmt.Sprintf("%s=gpu%d", cdiKind, g.minor)
}

type MigDeviceInfo struct {
	uuid    string
	parent  *GpuInfo
	profile *MigProfile
	giInfo  *nvml.GpuInstanceInfo
	ciInfo  *nvml.ComputeInstanceInfo
}

func (m MigDeviceInfo) CDIDevice() string {
	return fmt.Sprintf("%s=mig-gpu%d-gi%d-ci%d", cdiKind, m.parent.minor, m.giInfo.Id, m.ciInfo.Id)
}

type AllocatedDeviceInfo struct {
	gpu *GpuInfo
	mig *MigDeviceInfo
}

func (i AllocatedDeviceInfo) Type() string {
	if i.gpu != nil {
		return nvcrd.GpuDeviceType
	}
	if i.mig != nil {
		return nvcrd.MigDeviceType
	}
	return nvcrd.UnknownDeviceType
}

type MigProfileInfo struct {
	profile *MigProfile
	count   int
}

type UnallocatedDeviceInfo struct {
	*GpuInfo
	migProfiles map[string]*MigProfileInfo
}

type DeviceState struct {
	sync.Mutex
	cdi        cdiapi.Registry
	alldevices UnallocatedDevices
	available  UnallocatedDevices
	allocated  ClaimAllocations
}

func NewDeviceState(config *Config, nascrd *nvcrd.NodeAllocationState) (*DeviceState, error) {
	alldevices, err := enumerateAllPossibleDevices()
	if err != nil {
		return nil, fmt.Errorf("error enumerating all possible devices: %v", err)
	}

	cdi := cdiapi.GetRegistry(
		cdiapi.WithSpecDirs(cdiRoot),
	)

	err = cdi.Refresh()
	if err != nil {
		return nil, fmt.Errorf("unable to refresh the CDI registry: %v", err)
	}

	state := &DeviceState{
		cdi:        cdi,
		alldevices: alldevices,
		available:  alldevices.DeepCopy(),
		allocated:  make(ClaimAllocations),
	}

	err = state.SyncAllocatedFromCRDSpec(&nascrd.Spec)
	if err != nil {
		return nil, fmt.Errorf("unable to sync ClaimAllocations from CRD: %v", err)
	}

	for claimUid := range state.allocated {
		state.RemoveFromAvailable(state.allocated[claimUid])
	}

	return state, nil
}

func (s *DeviceState) Allocate(claimUid string, requirements nvcrd.DeviceRequirements) ([]string, error) {
	s.Lock()
	defer s.Unlock()

	if s.allocated[claimUid] != nil {
		return s.getAllocatedAsCDIDevices(claimUid), nil
	}

	var err error
	switch requirements.Type() {
	case nvcrd.GpuDeviceType:
		err = s.AllocateGpus(claimUid, requirements.Gpu)
	case nvcrd.MigDeviceType:
		err = s.AllocateMigDevices(claimUid, requirements.Mig)
	}
	if err != nil {
		return nil, fmt.Errorf("allocation failed: %v", err)
	}

	return s.getAllocatedAsCDIDevices(claimUid), nil
}

func (s *DeviceState) Free(claimUid string) error {
	s.Lock()
	defer s.Unlock()

	for _, device := range s.allocated[claimUid] {
		var err error
		switch device.Type() {
		case nvcrd.GpuDeviceType:
			break
		case nvcrd.MigDeviceType:
			err = s.FreeMigDevice(device.mig)
		}
		if err != nil {
			return fmt.Errorf("free failed: %v", err)
		}
	}

	s.AddtoAvailable(s.allocated[claimUid])
	delete(s.allocated, claimUid)
	return nil
}

func (s *DeviceState) AllocateGpus(claimUid string, requirements *nvcrd.GpuClaimSpec) error {
	var available []*GpuInfo
	for _, d := range s.available {
		if !d.migEnabled {
			available = append(available, d.GpuInfo)
		}
	}

	if len(available) < requirements.Count {
		return fmt.Errorf("not enough GPUs available for allocation (available: %v, required: %v)", len(available), requirements.Count)
	}

	allocated := make(AllocatedDevices)
	for _, gpu := range available[:requirements.Count] {
		allocated[gpu.uuid] = AllocatedDeviceInfo{
			gpu: gpu,
		}
	}

	s.allocated[claimUid] = allocated
	s.RemoveFromAvailable(allocated)

	return nil
}

func (s *DeviceState) AllocateMigDevices(claimUid string, requirements *nvcrd.MigDeviceClaimSpec) error {
	var gpus UnallocatedDevices
	for _, d := range s.available {
		if d.migEnabled {
			gpus = append(gpus, d)
		}
	}

	allocated := make(AllocatedDevices)
	for _, gpu := range gpus {
		if _, exists := gpu.migProfiles[requirements.Profile]; !exists {
			continue
		}
		if gpu.migProfiles[requirements.Profile].count == 0 {
			continue
		}

		migInfo, err := createMigDevice(gpu.GpuInfo, gpu.migProfiles[requirements.Profile].profile, nil)
		if err != nil {
			return fmt.Errorf("error creating MIG device: %v", err)
		}

		allocated[migInfo.uuid] = AllocatedDeviceInfo{
			mig: migInfo,
		}

		break
	}

	if len(allocated) == 0 {
		return fmt.Errorf("no MIG Devices with provided profile available on any GPUs: %v", requirements.Profile)
	}

	s.allocated[claimUid] = allocated
	s.RemoveFromAvailable(allocated)

	return nil
}

func (s *DeviceState) FreeMigDevice(mig *MigDeviceInfo) error {
	return deleteMigDevice(mig)
}

func (s *DeviceState) GetUpdatedSpec(inspec *nvcrd.NodeAllocationStateSpec) *nvcrd.NodeAllocationStateSpec {
	s.Lock()
	defer s.Unlock()

	outspec := inspec.DeepCopy()
	s.SyncAllDevicesToCRDSpec(outspec)
	s.SyncAllocatedToCRDSpec(outspec)
	return outspec
}

func (s *DeviceState) RemoveFromAvailable(ads AllocatedDevices) {
	newav := s.available.DeepCopy()
	(&newav).removeGpus(ads)
	(&newav).removeMigDevices(ads)
	s.available = newav
}

func (uds *UnallocatedDevices) removeGpus(ads AllocatedDevices) {
	var newuds UnallocatedDevices
	for _, ud := range *uds {
		if _, exists := ads[ud.uuid]; !exists {
			newuds = append(newuds, ud)
		}
	}
	*uds = newuds
}

func (uds *UnallocatedDevices) removeMigDevices(ads AllocatedDevices) {
	for _, ud := range *uds {
		for _, ad := range ads {
			if ad.Type() == nvcrd.MigDeviceType {
				if ud.uuid == ad.mig.parent.uuid {
					ud.migProfiles[ad.mig.profile.String()].count -= 1
				}
			}
		}
	}
}

func (s *DeviceState) AddtoAvailable(ads AllocatedDevices) {
	newav := s.available.DeepCopy()
	(&newav).addGpus(ads)
	(&newav).addMigDevices(ads)
	s.available = newav
}

func (uds *UnallocatedDevices) addGpus(ads AllocatedDevices) {
	for _, ad := range ads {
		if ad.Type() == nvcrd.GpuDeviceType {
			ud := UnallocatedDeviceInfo{
				GpuInfo: ad.gpu,
			}
			*uds = append(*uds, ud)
		}
	}
}

func (uds *UnallocatedDevices) addMigDevices(ads AllocatedDevices) {
	for _, ad := range ads {
		if ad.Type() == nvcrd.MigDeviceType {
			for _, ud := range *uds {
				if ud.uuid == ad.mig.parent.uuid {
					ud.migProfiles[ad.mig.profile.String()].count += 1
				}
			}
		}
	}
}

func (s *DeviceState) SyncAllDevicesToCRDSpec(spec *nvcrd.NodeAllocationStateSpec) {
	gpus := make(map[string]nvcrd.AllocatableDevice)
	migs := make(map[string]nvcrd.AllocatableDevice)
	for _, device := range s.alldevices {
		if _, exists := gpus[device.name]; !exists {
			gpus[device.name] = nvcrd.AllocatableDevice{
				Gpu: &nvcrd.AllocatableGpu{
					Name:  device.name,
					Count: 0,
				},
			}
		}

		if !device.migEnabled {
			gpus[device.name].Gpu.Count += 1
			continue
		}

		for _, mig := range device.migProfiles {
			if _, exists := migs[mig.profile.String()]; !exists {
				migs[mig.profile.String()] = nvcrd.AllocatableDevice{
					Mig: &nvcrd.AllocatableMigDevice{
						Profile: mig.profile.String(),
						Count:   0,
						Slices:  mig.profile.G,
					},
				}
			}
			migs[mig.profile.String()].Mig.Count += mig.count
		}
	}

	allocatable := []nvcrd.AllocatableDevice{}
	for _, device := range gpus {
		if device.Gpu.Count > 0 {
			allocatable = append(allocatable, device)
		}
	}
	for _, device := range migs {
		if device.Mig.Count > 0 {
			allocatable = append(allocatable, device)
		}
	}

	spec.AllocatableDevices = allocatable
}

func (s *DeviceState) SyncAllocatedFromCRDSpec(spec *nvcrd.NodeAllocationStateSpec) error {
	//outcas := make(ClaimAllocations)
	//for claim, devices := range incas {
	//	outcas[claim] = make(AllocatedDevices)
	//	for _, d := range devices {
	//		switch d.Type() {
	//		case nvcrd.GpuDeviceType:
	//			outcas[claim][d.Gpu.UUID].Insert(d.Gpu.CDIDeviceName)
	//		}
	//	}
	//}
	//for claim := range cas {
	//	delete(cas, claim)
	//}
	//for claim := range outcas {
	//	cas[claim] = outcas[claim]
	//}
	return nil
}

func (s *DeviceState) SyncAllocatedToCRDSpec(spec *nvcrd.NodeAllocationStateSpec) {
	outcas := make(map[string][]nvcrd.AllocatedDevice)
	for claim, devices := range s.allocated {
		var allocated []nvcrd.AllocatedDevice
		for uuid, device := range devices {
			outdevice := nvcrd.AllocatedDevice{}
			switch device.Type() {
			case nvcrd.GpuDeviceType:
				outdevice.Gpu = &nvcrd.AllocatedGpu{
					UUID:      uuid,
					Name:      device.gpu.name,
					CDIDevice: device.gpu.CDIDevice(),
				}
			case nvcrd.MigDeviceType:
				outdevice.Mig = &nvcrd.AllocatedMigDevice{
					UUID:      uuid,
					Profile:   device.mig.profile.String(),
					CDIDevice: device.mig.CDIDevice(),
					Placement: nvcrd.MigDevicePlacement{
						GpuUUID: device.mig.parent.uuid,
						Start:   int(device.mig.giInfo.Placement.Start),
						Size:    int(device.mig.giInfo.Placement.Size),
					},
				}
			}
			allocated = append(allocated, outdevice)
		}
		outcas[claim] = allocated
	}
	spec.ClaimAllocations = outcas
}

func (s *DeviceState) getAllocatedAsCDIDevices(claimUid string) []string {
	var devs []string
	for _, device := range s.allocated[claimUid] {
		var cdiDevice string
		switch device.Type() {
		case nvcrd.GpuDeviceType:
			cdiDevice = device.gpu.CDIDevice()
		case nvcrd.MigDeviceType:
			cdiDevice = device.mig.CDIDevice()
		}
		devs = append(devs, s.cdi.DeviceDB().GetDevice(cdiDevice).GetQualifiedName())
	}
	return devs
}

func (ds UnallocatedDevices) DeepCopy() UnallocatedDevices {
	var newds UnallocatedDevices
	for _, d := range ds {
		gpuInfo := *d.GpuInfo

		migProfiles := make(map[string]*MigProfileInfo)
		for _, p := range d.migProfiles {
			newp := &MigProfileInfo{
				profile: p.profile,
				count:   p.count,
			}
			copy(newp.possiblePlacements, p.possiblePlacements)
			migProfiles[p.profile.String()] = newp
		}

		deviceInfo := UnallocatedDeviceInfo{
			GpuInfo:     &gpuInfo,
			migProfiles: migProfiles,
		}

		newds = append(newds, deviceInfo)
	}
	return newds
}