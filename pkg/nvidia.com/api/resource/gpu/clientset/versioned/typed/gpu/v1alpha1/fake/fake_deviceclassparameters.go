/*
 * Copyright (c) 2023, NVIDIA CORPORATION.  All rights reserved.
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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/NVIDIA/k8s-dra-driver/pkg/nvidia.com/api/resource/gpu/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeDeviceClassParameters implements DeviceClassParametersInterface
type FakeDeviceClassParameters struct {
	Fake *FakeGpuV1alpha1
}

var deviceclassparametersResource = schema.GroupVersionResource{Group: "gpu.resource.nvidia.com", Version: "v1alpha1", Resource: "deviceclassparameters"}

var deviceclassparametersKind = schema.GroupVersionKind{Group: "gpu.resource.nvidia.com", Version: "v1alpha1", Kind: "DeviceClassParameters"}

// Get takes name of the deviceClassParameters, and returns the corresponding deviceClassParameters object, and an error if there is any.
func (c *FakeDeviceClassParameters) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.DeviceClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(deviceclassparametersResource, name), &v1alpha1.DeviceClassParameters{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DeviceClassParameters), err
}

// List takes label and field selectors, and returns the list of DeviceClassParameters that match those selectors.
func (c *FakeDeviceClassParameters) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.DeviceClassParametersList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(deviceclassparametersResource, deviceclassparametersKind, opts), &v1alpha1.DeviceClassParametersList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.DeviceClassParametersList{ListMeta: obj.(*v1alpha1.DeviceClassParametersList).ListMeta}
	for _, item := range obj.(*v1alpha1.DeviceClassParametersList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested deviceClassParameters.
func (c *FakeDeviceClassParameters) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(deviceclassparametersResource, opts))
}

// Create takes the representation of a deviceClassParameters and creates it.  Returns the server's representation of the deviceClassParameters, and an error, if there is any.
func (c *FakeDeviceClassParameters) Create(ctx context.Context, deviceClassParameters *v1alpha1.DeviceClassParameters, opts v1.CreateOptions) (result *v1alpha1.DeviceClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(deviceclassparametersResource, deviceClassParameters), &v1alpha1.DeviceClassParameters{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DeviceClassParameters), err
}

// Update takes the representation of a deviceClassParameters and updates it. Returns the server's representation of the deviceClassParameters, and an error, if there is any.
func (c *FakeDeviceClassParameters) Update(ctx context.Context, deviceClassParameters *v1alpha1.DeviceClassParameters, opts v1.UpdateOptions) (result *v1alpha1.DeviceClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(deviceclassparametersResource, deviceClassParameters), &v1alpha1.DeviceClassParameters{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DeviceClassParameters), err
}

// Delete takes name of the deviceClassParameters and deletes it. Returns an error if one occurs.
func (c *FakeDeviceClassParameters) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(deviceclassparametersResource, name, opts), &v1alpha1.DeviceClassParameters{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeDeviceClassParameters) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(deviceclassparametersResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.DeviceClassParametersList{})
	return err
}

// Patch applies the patch and returns the patched deviceClassParameters.
func (c *FakeDeviceClassParameters) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.DeviceClassParameters, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(deviceclassparametersResource, name, pt, data, subresources...), &v1alpha1.DeviceClassParameters{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.DeviceClassParameters), err
}
