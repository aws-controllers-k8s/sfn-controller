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

package state_machine_alias

import (
	"context"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/sfn"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// customUpdateStateMachineAlias patches each of the resource properties in the backend AWS
// service API and returns a new resource with updated fields.
func (rm *resourceManager) customUpdateStateMachineAlias(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customUpdateStateMachineAlias")
	defer exit(nil)

	input := &svcsdk.UpdateStateMachineAliasInput{}

	if desired.ko.Status.ACKResourceMetadata != nil && desired.ko.Status.ACKResourceMetadata.ARN != nil {
		arnCopy := string(*desired.ko.Status.ACKResourceMetadata.ARN)
		input.StateMachineAliasArn = &arnCopy
	}
	if desired.ko.Spec.Description != nil {
		input.Description = desired.ko.Spec.Description
	}
	if desired.ko.Spec.RoutingConfiguration != nil {
		f0 := []svcsdktypes.RoutingConfigurationListItem{}
		for _, f0iter := range desired.ko.Spec.RoutingConfiguration {
			f0elem := svcsdktypes.RoutingConfigurationListItem{}
			if f0iter.StateMachineVersionARN != nil {
				f0elem.StateMachineVersionArn = f0iter.StateMachineVersionARN
			}
			if f0iter.Weight != nil {
				f0elem.Weight = int32(*f0iter.Weight)
			}
			f0 = append(f0, f0elem)
		}
		input.RoutingConfiguration = f0
	}

	resp, err := rm.sdkapi.UpdateStateMachineAlias(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UpdateStateMachineAlias", err)
	if err != nil {
		return nil, err
	}

	ko := desired.ko.DeepCopy()
	if resp.UpdateDate != nil {
		ko.Status.UpdateDate = &metav1.Time{*resp.UpdateDate}
	}

	rm.setStatusDefaults(ko)
	return &resource{ko}, nil
}
