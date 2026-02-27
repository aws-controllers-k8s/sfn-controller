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

package v1alpha1

import (
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StateMachineVersionSpec defines the desired state of StateMachineVersion.
type StateMachineVersionSpec struct {
	// The Amazon Resource Name (ARN) of the state machine to publish a version of.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable once set"
	StateMachineARN *string `json:"stateMachineARN,omitempty"`
	// AWSResourceReferenceWrapper provides a wrapper around *AWSResourceReference
	// type to provide more user friendly syntax for references using 'from' field
	// Ex:
	// APIIDRef:
	//
	//	from:
	//	  name: my-api
	StateMachineRef *ackv1alpha1.AWSResourceReferenceWrapper `json:"stateMachineRef,omitempty"`
	// An optional description of the state machine version.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable once set"
	Description *string `json:"description,omitempty"`
	// Only publish the state machine version if the current state machine's revision
	// ID matches the specified ID.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable once set"
	RevisionID *string `json:"revisionID,omitempty"`
}

// StateMachineVersionStatus defines the observed state of StateMachineVersion
type StateMachineVersionStatus struct {
	// All CRs managed by ACK have a common `Status.ACKResourceMetadata` member
	// that is used to contain resource sync state, account ownership,
	// constructed ARN for the resource
	// +kubebuilder:validation:Optional
	ACKResourceMetadata *ackv1alpha1.ResourceMetadata `json:"ackResourceMetadata"`
	// All CRs managed by ACK have a common `Status.Conditions` member that
	// contains a collection of `ackv1alpha1.Condition` objects that describe
	// the various terminal states of the CR and its backend AWS service API
	// resource
	// +kubebuilder:validation:Optional
	Conditions []*ackv1alpha1.Condition `json:"conditions"`
	// The date the state machine version was created.
	// +kubebuilder:validation:Optional
	CreationDate *metav1.Time `json:"creationDate,omitempty"`
	// The ARN of the published state machine version.
	// +kubebuilder:validation:Optional
	StateMachineVersionARN *string `json:"stateMachineVersionARN,omitempty"`
}

// StateMachineVersion is the Schema for the StateMachineVersions API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type StateMachineVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              StateMachineVersionSpec   `json:"spec,omitempty"`
	Status            StateMachineVersionStatus `json:"status,omitempty"`
}

// StateMachineVersionList contains a list of StateMachineVersion
// +kubebuilder:object:root=true
type StateMachineVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StateMachineVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StateMachineVersion{}, &StateMachineVersionList{})
}
