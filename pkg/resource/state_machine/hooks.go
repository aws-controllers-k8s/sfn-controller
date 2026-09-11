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
	"errors"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/sfn"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"
	smithy "github.com/aws/smithy-go"

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

	rm.clearDeletedVersionARN(ctx, ko)

	return nil
}

// clearDeletedVersionARN drops Status.StateMachineVersionARN when the version it
// names no longer exists.
//
// DescribeStateMachine does not report the field, so it would otherwise survive
// the read untouched and keep naming a version deleted out of band. Clearing it
// is what tells customPreCompare a publish is owed: a nil ARN under
// Spec.Publish means the configuration last pushed is held by no version, which
// covers both a state machine that has never published and one whose version
// was removed behind the controller's back. Content changes need none of this,
// they already produce their own Spec delta.
//
// The probe is deliberately scoped to this function. A missing version answers
// with StateMachineDoesNotExist, the same code this resource maps to 404, so
// letting it escape would make the read report the state machine itself as
// gone. Only that code clears the ARN; any other failure, throttling for
// instance, leaves it alone rather than manufacturing a publish.
func (rm *resourceManager) clearDeletedVersionARN(
	ctx context.Context,
	ko *svcapitypes.StateMachine,
) {
	// Nothing to probe: no version is recorded, or the user does not version at
	// all and the extra read would be pure overhead.
	if ko.Spec.Publish == nil || !*ko.Spec.Publish ||
		ko.Status.StateMachineVersionARN == nil {
		return
	}
	_, err := rm.sdkapi.DescribeStateMachine(ctx, &svcsdk.DescribeStateMachineInput{
		StateMachineArn: ko.Status.StateMachineVersionARN,
	})
	var apiErr smithy.APIError
	versionGone := errors.As(err, &apiErr) &&
		apiErr.ErrorCode() == "StateMachineDoesNotExist"

	// Recorded under its own operation type so this probe stays separable from
	// the resource's own read, which calls the same API and would otherwise
	// absorb it. A missing version is the answer the probe is looking for
	// rather than a failure, so it counts as a success: the error counter is
	// keyed on the API name alone, so reporting it would show expected 404s
	// against DescribeStateMachine and make ordinary reads look broken.
	callErr := err
	if versionGone {
		callErr = nil
	}
	rm.metrics.RecordAPICall("GET", "DescribeStateMachine", callErr)

	if versionGone {
		ko.Status.StateMachineVersionARN = nil
	}
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
		resp, err := rm.updateStateMachine(ctx, desired)
		if err != nil {
			return nil, err
		}
		// Mirror the response, including its absence. StateMachineVersionArn
		// comes back only when this call published, so clearing it otherwise
		// keeps the field meaning "the version holding the configuration we
		// last pushed" -- which is exactly what customPreCompare reads to
		// decide a publish is owed. Left stale, a later flip of Spec.Publish
		// would find a version that exists but no longer matches the state
		// machine, and publish nothing.
		desired.ko.Status.StateMachineVersionARN = resp.StateMachineVersionArn
	}

	rm.setStatusDefaults(desired.ko)
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
	// A publish is owed: the user asked for versions but no version holds the
	// configuration last pushed. Registered under a Spec path because the
	// runtime only calls Update when delta.DifferentAt("Spec") holds; anything
	// else would be recorded and then never acted on. The values are the
	// recorded ARNs rather than Publish itself, which compares equal on both
	// sides -- latest starts as a copy of desired and DescribeStateMachine has
	// no Publish member to overwrite it.
	if a.ko.Spec.Publish != nil && *a.ko.Spec.Publish &&
		b.ko.Status.StateMachineVersionARN == nil {
		delta.Add("Spec.Publish",
			a.ko.Status.StateMachineVersionARN,
			b.ko.Status.StateMachineVersionARN)
	}
}

// sdkUpdate patches the supplied resource in the backend AWS service API and
// returns a new resource with updated fields.
func (rm *resourceManager) updateStateMachine(
	ctx context.Context,
	desired *resource,
) (resp *svcsdk.UpdateStateMachineOutput, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkUpdate")
	defer func() {
		exit(err)
	}()
	input, err := rm.newUpdateRequestPayload(ctx, desired)
	if err != nil {
		return nil, err
	}

	resp, err = rm.sdkapi.UpdateStateMachine(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UpdateStateMachine", err)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// newUpdateRequestPayload returns an SDK-specific struct for the HTTP request
// payload of the Update API call for the resource
//
// This is a manual copy of the generated payload builder: update_operation
// carries a custom_method_name, so code generation emits only a thin sdkUpdate
// wrapper and never regenerates this function. Every Spec field that belongs on
// UpdateStateMachineInput has to be added here by hand -- a field that reaches
// the Create payload automatically will silently be missing from Update
// otherwise.
func (rm *resourceManager) newUpdateRequestPayload(
	ctx context.Context,
	r *resource,
) (*svcsdk.UpdateStateMachineInput, error) {
	res := &svcsdk.UpdateStateMachineInput{}

	if r.ko.Spec.Definition != nil {
		res.Definition = r.ko.Spec.Definition
	}
	if r.ko.Spec.LoggingConfiguration != nil {
		f1 := &svcsdktypes.LoggingConfiguration{}
		if r.ko.Spec.LoggingConfiguration.Destinations != nil {
			f1f0 := []svcsdktypes.LogDestination{}
			for _, f1f0iter := range r.ko.Spec.LoggingConfiguration.Destinations {
				f1f0elem := svcsdktypes.LogDestination{}
				if f1f0iter.CloudWatchLogsLogGroup != nil {
					f1f0elemf0 := &svcsdktypes.CloudWatchLogsLogGroup{}
					if f1f0iter.CloudWatchLogsLogGroup.LogGroupARN != nil {
						f1f0elemf0.LogGroupArn = f1f0iter.CloudWatchLogsLogGroup.LogGroupARN
					}
					f1f0elem.CloudWatchLogsLogGroup = f1f0elemf0
				}
				f1f0 = append(f1f0, f1f0elem)
			}
			f1.Destinations = f1f0
		}
		if r.ko.Spec.LoggingConfiguration.IncludeExecutionData != nil {
			f1.IncludeExecutionData = *r.ko.Spec.LoggingConfiguration.IncludeExecutionData
		}
		if r.ko.Spec.LoggingConfiguration.Level != nil {
			f1.Level = svcsdktypes.LogLevel(*r.ko.Spec.LoggingConfiguration.Level)
		}
		res.LoggingConfiguration = f1
	}
	if r.ko.Spec.Publish != nil {
		res.Publish = *r.ko.Spec.Publish
	}
	if r.ko.Spec.RoleARN != nil {
		res.RoleArn = r.ko.Spec.RoleARN
	}
	if r.ko.Status.ACKResourceMetadata != nil && r.ko.Status.ACKResourceMetadata.ARN != nil {
		arnCopy := string(*r.ko.Status.ACKResourceMetadata.ARN)
		res.StateMachineArn = &arnCopy
	}
	if r.ko.Spec.TracingConfiguration != nil {
		f4 := &svcsdktypes.TracingConfiguration{}
		if r.ko.Spec.TracingConfiguration.Enabled != nil {
			f4.Enabled = *r.ko.Spec.TracingConfiguration.Enabled
		}
		res.TracingConfiguration = f4
	}

	return res, nil
}
