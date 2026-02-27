// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package state_machine_version

import (
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	acktypes "github.com/aws-controllers-k8s/runtime/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	rtclient "sigs.k8s.io/controller-runtime/pkg/client"
	k8sctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	svcapitypes "github.com/aws-controllers-k8s/sfn-controller/apis/v1alpha1"
)

const (
	FinalizerString = "finalizers.sfn.services.k8s.aws/StateMachineVersion"
)

var (
	GroupVersionResource = svcapitypes.GroupVersion.WithResource("statemachineversions")
	GroupKind            = metav1.GroupKind{
		Group: "sfn.services.k8s.aws",
		Kind:  "StateMachineVersion",
	}
)

// resourceDescriptor implements the
// `aws-service-operator-k8s/pkg/types.AWSResourceDescriptor` interface
type resourceDescriptor struct {
}

// GroupVersionKind returns a Kubernetes schema.GroupVersionKind struct that
// describes the API Group, Version and Kind of CRs described by the descriptor
func (d *resourceDescriptor) GroupVersionKind() schema.GroupVersionKind {
	return svcapitypes.GroupVersion.WithKind(GroupKind.Kind)
}

// EmptyRuntimeObject returns an empty object prototype that may be used in
// apimachinery and k8s client operations
func (d *resourceDescriptor) EmptyRuntimeObject() rtclient.Object {
	return &svcapitypes.StateMachineVersion{}
}

// ResourceFromRuntimeObject returns an AWSResource that has been initialized
// with the supplied runtime.Object
func (d *resourceDescriptor) ResourceFromRuntimeObject(
	obj rtclient.Object,
) acktypes.AWSResource {
	return &resource{
		ko: obj.(*svcapitypes.StateMachineVersion),
	}
}

// Delta returns an `ackcompare.Delta` object containing the difference between
// one `AWSResource` and another.
func (d *resourceDescriptor) Delta(a, b acktypes.AWSResource) *ackcompare.Delta {
	return newResourceDelta(a.(*resource), b.(*resource))
}

// IsManaged returns true if the supplied AWSResource is under the management
// of an ACK service controller.
func (d *resourceDescriptor) IsManaged(
	res acktypes.AWSResource,
) bool {
	obj := res.RuntimeObject()
	if obj == nil {
		panic("nil RuntimeMetaObject in AWSResource")
	}
	return containsFinalizer(obj, FinalizerString)
}

func containsFinalizer(obj rtclient.Object, finalizer string) bool {
	f := obj.GetFinalizers()
	for _, e := range f {
		if e == finalizer {
			return true
		}
	}
	return false
}

// MarkManaged places the supplied resource under the management of ACK.
func (d *resourceDescriptor) MarkManaged(
	res acktypes.AWSResource,
) {
	obj := res.RuntimeObject()
	if obj == nil {
		panic("nil RuntimeMetaObject in AWSResource")
	}
	k8sctrlutil.AddFinalizer(obj, FinalizerString)
}

// MarkUnmanaged removes the supplied resource from management by ACK.
func (d *resourceDescriptor) MarkUnmanaged(
	res acktypes.AWSResource,
) {
	obj := res.RuntimeObject()
	if obj == nil {
		panic("nil RuntimeMetaObject in AWSResource")
	}
	k8sctrlutil.RemoveFinalizer(obj, FinalizerString)
}

// MarkAdopted places descriptors on the custom resource that indicate the
// resource was not created from within ACK.
func (d *resourceDescriptor) MarkAdopted(
	res acktypes.AWSResource,
) {
	obj := res.RuntimeObject()
	if obj == nil {
		panic("nil RuntimeObject in AWSResource")
	}
	curr := obj.GetAnnotations()
	if curr == nil {
		curr = make(map[string]string)
	}
	curr[ackv1alpha1.AnnotationAdopted] = "true"
	obj.SetAnnotations(curr)
}
