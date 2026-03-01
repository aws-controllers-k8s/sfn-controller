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
	"errors"
	"fmt"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/sfn"
	smithy "github.com/aws/smithy-go"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// customFindStateMachineVersion uses DescribeStateMachine with the version ARN
// to read a specific state machine version. There is no dedicated
// DescribeStateMachineVersion API, so we call DescribeStateMachine with the
// version ARN (which includes the version number suffix).
func (rm *resourceManager) customFindStateMachineVersion(
	ctx context.Context,
	r *resource,
) (*resource, error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customFindStateMachineVersion")
	defer exit(nil)

	if r.ko.Status.ACKResourceMetadata == nil || r.ko.Status.ACKResourceMetadata.ARN == nil {
		return nil, ackerr.NotFound
	}

	input := &svcsdk.DescribeStateMachineInput{}
	input.StateMachineArn = (*string)(r.ko.Status.ACKResourceMetadata.ARN)

	resp, err := rm.sdkapi.DescribeStateMachine(ctx, input)
	rm.metrics.RecordAPICall("READ_ONE", "DescribeStateMachine", err)
	if err != nil {
		var awsErr smithy.APIError
		if errors.As(err, &awsErr) && awsErr.ErrorCode() == "StateMachineDoesNotExist" {
			return nil, ackerr.NotFound
		}
		return nil, err
	}

	ko := r.ko.DeepCopy()

	if resp.CreationDate != nil {
		ko.Status.CreationDate = &metav1.Time{Time: *resp.CreationDate}
	} else {
		ko.Status.CreationDate = nil
	}
	if resp.Description != nil {
		ko.Spec.Description = resp.Description
	}
	if resp.RevisionId != nil {
		ko.Spec.RevisionID = resp.RevisionId
	}
	if resp.StateMachineArn != nil {
		arn := ackv1alpha1.AWSResourceName(*resp.StateMachineArn)
		ko.Status.ACKResourceMetadata.ARN = &arn
	}

	rm.setStatusDefaults(ko)
	return &resource{ko}, nil
}

// customUpdateStateMachineVersion returns a terminal error because state machine
// versions are immutable and cannot be updated.
func (rm *resourceManager) customUpdateStateMachineVersion(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (*resource, error) {
	return nil, ackerr.NewTerminalError(
		fmt.Errorf("state machine versions are immutable and cannot be updated"),
	)
}
