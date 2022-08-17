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

package state_machine

import (
	"context"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go/service/sfn"

	svcapitypes "github.com/aws-controllers-k8s/sfn-controller/apis/v1alpha1"
	commonutil "github.com/aws-controllers-k8s/sfn-controller/pkg/util"
)

// setResourceAdditionalFields queries and adds the tags to a StateMachine resource
func (rm *resourceManager) setResourceAdditionalFields(
	ctx context.Context,
	ko *svcapitypes.StateMachine,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.setResourceAdditionalFields")
	defer exit(err)

	// Set StateMachine tags
	ko.Spec.Tags, err = commonutil.GetResourceTags(
		ctx,
		rm.sdkapi,
		rm.metrics,
		string(*ko.Status.ACKResourceMetadata.ARN),
	)
	if err != nil {
		return err
	}

	return nil
}

// customUpdateStateMachine patches each of the resource properties in the backend AWS
// service API and returns a new resource with updated fields.
func (rm *resourceManager) customUpdateStateMachine(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {
	if delta.DifferentAt("Spec.Tags") {
		err := commonutil.SyncResourceTags(
			ctx,
			rm.sdkapi,
			rm.metrics,
			string(*desired.ko.Status.ACKResourceMetadata.ARN),
			latest.ko.Spec.Tags,
			desired.ko.Spec.Tags,
		)
		if err != nil {
			return nil, err
		}
	}
	if delta.DifferentExcept("Spec.Tags") {
		err := rm.updateStateMachine(ctx, desired)
		if err != nil {
			return nil, err
		}
	}
	return desired, nil
}

func customPreCompare(
	delta *ackcompare.Delta,
	a *resource,
	b *resource,
) {
	if len(a.ko.Spec.Tags) != len(b.ko.Spec.Tags) {
		delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
	} else if len(a.ko.Spec.Tags) > 0 {
		if !commonutil.EqualTags(a.ko.Spec.Tags, b.ko.Spec.Tags) {
			delta.Add("Spec.Tags", a.ko.Spec.Tags, b.ko.Spec.Tags)
		}
	}
}

// sdkUpdate patches the supplied resource in the backend AWS service API and
// returns a new resource with updated fields.
func (rm *resourceManager) updateStateMachine(
	ctx context.Context,
	desired *resource,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkUpdate")
	defer func() {
		exit(err)
	}()
	input, err := rm.newUpdateRequestPayload(ctx, desired)
	if err != nil {
		return err
	}

	_, err = rm.sdkapi.UpdateStateMachineWithContext(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UpdateStateMachine", err)
	if err != nil {
		return err
	}
	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := desired.ko.DeepCopy()

	rm.setStatusDefaults(ko)
	return nil
}

// newUpdateRequestPayload returns an SDK-specific struct for the HTTP request
// payload of the Update API call for the resource
func (rm *resourceManager) newUpdateRequestPayload(
	ctx context.Context,
	r *resource,
) (*svcsdk.UpdateStateMachineInput, error) {
	res := &svcsdk.UpdateStateMachineInput{}

	if r.ko.Spec.Definition != nil {
		res.SetDefinition(*r.ko.Spec.Definition)
	}
	if r.ko.Spec.LoggingConfiguration != nil {
		f1 := &svcsdk.LoggingConfiguration{}
		if r.ko.Spec.LoggingConfiguration.Destinations != nil {
			f1f0 := []*svcsdk.LogDestination{}
			for _, f1f0iter := range r.ko.Spec.LoggingConfiguration.Destinations {
				f1f0elem := &svcsdk.LogDestination{}
				if f1f0iter.CloudWatchLogsLogGroup != nil {
					f1f0elemf0 := &svcsdk.CloudWatchLogsLogGroup{}
					if f1f0iter.CloudWatchLogsLogGroup.LogGroupARN != nil {
						f1f0elemf0.SetLogGroupArn(*f1f0iter.CloudWatchLogsLogGroup.LogGroupARN)
					}
					f1f0elem.SetCloudWatchLogsLogGroup(f1f0elemf0)
				}
				f1f0 = append(f1f0, f1f0elem)
			}
			f1.SetDestinations(f1f0)
		}
		if r.ko.Spec.LoggingConfiguration.IncludeExecutionData != nil {
			f1.SetIncludeExecutionData(*r.ko.Spec.LoggingConfiguration.IncludeExecutionData)
		}
		if r.ko.Spec.LoggingConfiguration.Level != nil {
			f1.SetLevel(*r.ko.Spec.LoggingConfiguration.Level)
		}
		res.SetLoggingConfiguration(f1)
	}
	if r.ko.Spec.RoleARN != nil {
		res.SetRoleArn(*r.ko.Spec.RoleARN)
	}
	if r.ko.Status.ACKResourceMetadata != nil && r.ko.Status.ACKResourceMetadata.ARN != nil {
		res.SetStateMachineArn(string(*r.ko.Status.ACKResourceMetadata.ARN))
	}
	if r.ko.Spec.TracingConfiguration != nil {
		f4 := &svcsdk.TracingConfiguration{}
		if r.ko.Spec.TracingConfiguration.Enabled != nil {
			f4.SetEnabled(*r.ko.Spec.TracingConfiguration.Enabled)
		}
		res.SetTracingConfiguration(f4)
	}

	return res, nil
}
