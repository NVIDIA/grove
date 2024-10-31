/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "github.com/NVIDIA/grove/operator/api/podgangset/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakePodGangSets implements PodGangSetInterface
type FakePodGangSets struct {
	Fake *FakeCoreV1alpha1
	ns   string
}

var podgangsetsResource = v1alpha1.SchemeGroupVersion.WithResource("podgangsets")

var podgangsetsKind = v1alpha1.SchemeGroupVersion.WithKind("PodGangSet")

// Get takes name of the podGangSet, and returns the corresponding podGangSet object, and an error if there is any.
func (c *FakePodGangSets) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.PodGangSet, err error) {
	emptyResult := &v1alpha1.PodGangSet{}
	obj, err := c.Fake.
		Invokes(testing.NewGetActionWithOptions(podgangsetsResource, c.ns, name, options), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.PodGangSet), err
}

// List takes label and field selectors, and returns the list of PodGangSets that match those selectors.
func (c *FakePodGangSets) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.PodGangSetList, err error) {
	emptyResult := &v1alpha1.PodGangSetList{}
	obj, err := c.Fake.
		Invokes(testing.NewListActionWithOptions(podgangsetsResource, podgangsetsKind, c.ns, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.PodGangSetList{ListMeta: obj.(*v1alpha1.PodGangSetList).ListMeta}
	for _, item := range obj.(*v1alpha1.PodGangSetList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested podGangSets.
func (c *FakePodGangSets) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchActionWithOptions(podgangsetsResource, c.ns, opts))

}

// Create takes the representation of a podGangSet and creates it.  Returns the server's representation of the podGangSet, and an error, if there is any.
func (c *FakePodGangSets) Create(ctx context.Context, podGangSet *v1alpha1.PodGangSet, opts v1.CreateOptions) (result *v1alpha1.PodGangSet, err error) {
	emptyResult := &v1alpha1.PodGangSet{}
	obj, err := c.Fake.
		Invokes(testing.NewCreateActionWithOptions(podgangsetsResource, c.ns, podGangSet, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.PodGangSet), err
}

// Update takes the representation of a podGangSet and updates it. Returns the server's representation of the podGangSet, and an error, if there is any.
func (c *FakePodGangSets) Update(ctx context.Context, podGangSet *v1alpha1.PodGangSet, opts v1.UpdateOptions) (result *v1alpha1.PodGangSet, err error) {
	emptyResult := &v1alpha1.PodGangSet{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateActionWithOptions(podgangsetsResource, c.ns, podGangSet, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.PodGangSet), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePodGangSets) UpdateStatus(ctx context.Context, podGangSet *v1alpha1.PodGangSet, opts v1.UpdateOptions) (result *v1alpha1.PodGangSet, err error) {
	emptyResult := &v1alpha1.PodGangSet{}
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceActionWithOptions(podgangsetsResource, "status", c.ns, podGangSet, opts), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.PodGangSet), err
}

// Delete takes name of the podGangSet and deletes it. Returns an error if one occurs.
func (c *FakePodGangSets) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteActionWithOptions(podgangsetsResource, c.ns, name, opts), &v1alpha1.PodGangSet{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePodGangSets) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionActionWithOptions(podgangsetsResource, c.ns, opts, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.PodGangSetList{})
	return err
}

// Patch applies the patch and returns the patched podGangSet.
func (c *FakePodGangSets) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.PodGangSet, err error) {
	emptyResult := &v1alpha1.PodGangSet{}
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceActionWithOptions(podgangsetsResource, c.ns, name, pt, data, opts, subresources...), emptyResult)

	if obj == nil {
		return emptyResult, err
	}
	return obj.(*v1alpha1.PodGangSet), err
}