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
	"context"
	"fmt"

	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	acktypes "github.com/aws-controllers-k8s/runtime/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8stypes "k8s.io/apimachinery/pkg/types"

	svcapitypes "github.com/aws-controllers-k8s/sfn-controller/apis/v1alpha1"
)

// ClearResolvedReferences removes any reference values that were made
// concrete in the spec. It returns a copy of the input AWSResource which
// contains the original *Ref values, but none of their respective concrete
// values.
func (rm *resourceManager) ClearResolvedReferences(res acktypes.AWSResource) acktypes.AWSResource {
	ko := rm.concreteResource(res).ko.DeepCopy()
	if ko.Spec.StateMachineRef != nil && ko.Spec.StateMachineRef.From != nil {
		ko.Spec.StateMachineARN = nil
	}
	return &resource{ko}
}

// ResolveReferences finds if there are any Reference field(s) present
// inside AWSResource passed in the parameter and attempts to resolve those
// reference field(s) into their respective target field(s).
func (rm *resourceManager) ResolveReferences(
	ctx context.Context,
	apiReader client.Reader,
	res acktypes.AWSResource,
) (acktypes.AWSResource, bool, error) {
	ko := rm.concreteResource(res).ko

	hasReferences := false
	resolvedKO := ko.DeepCopy()

	if ko.Spec.StateMachineRef != nil && ko.Spec.StateMachineRef.From != nil {
		hasReferences = true
		arr, err := resolveReferenceForStateMachineARN(ctx, apiReader, ko)
		if err != nil {
			return nil, hasReferences, err
		}
		resolvedKO.Spec.StateMachineARN = arr
	}

	return &resource{resolvedKO}, hasReferences, nil
}

// resolveReferenceForStateMachineARN reads the resource referenced
// from StateMachineRef field and sets the StateMachineARN from
// referenced resource's Status.ACKResourceMetadata.ARN
func resolveReferenceForStateMachineARN(
	ctx context.Context,
	apiReader client.Reader,
	ko *svcapitypes.StateMachineVersion,
) (*string, error) {
	if ko.Spec.StateMachineRef != nil && ko.Spec.StateMachineRef.From != nil {
		ref := ko.Spec.StateMachineRef.From
		if ref.Name == nil || *ref.Name == "" {
			return nil, fmt.Errorf("provided resource reference is nil or empty: StateMachineRef")
		}
		refName := *ref.Name

		ns := ko.Namespace
		if ref.Namespace != nil && *ref.Namespace != "" {
			ns = *ref.Namespace
		}

		obj := &svcapitypes.StateMachine{}
		if err := apiReader.Get(ctx, k8stypes.NamespacedName{Namespace: ns, Name: refName}, obj); err != nil {
			if client.IgnoreNotFound(err) != nil {
				return nil, err
			}
			return nil, ackerr.ResourceReferenceTerminalFor(
				"StateMachine",
				ns, refName,
			)
		}
		arn := (*string)(obj.Status.ACKResourceMetadata.ARN)
		if arn == nil {
			return nil, ackerr.ResourceReferenceMissingTargetFieldFor(
				"StateMachine",
				ns, refName,
				"Status.ACKResourceMetadata.ARN",
			)
		}
		return arn, nil
	}

	return ko.Spec.StateMachineARN, nil
}

// validateReferenceFields validates the reference field and corresponding
// identifier field.
func validateReferenceFields(ko *svcapitypes.StateMachineVersion) error {
	if ko.Spec.StateMachineRef != nil && ko.Spec.StateMachineRef.From != nil {
		if ko.Spec.StateMachineARN != nil {
			return ackerr.ResourceReferenceAndIDNotSupportedFor("StateMachineARN", "StateMachineRef")
		}
	}
	if ko.Spec.StateMachineRef == nil && ko.Spec.StateMachineARN == nil {
		return ackerr.ResourceReferenceOrIDRequiredFor("StateMachineARN", "StateMachineRef")
	}
	return nil
}
