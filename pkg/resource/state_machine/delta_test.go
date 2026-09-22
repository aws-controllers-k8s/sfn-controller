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
	"fmt"
	"testing"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	"github.com/aws/aws-sdk-go-v2/aws"

	svcapitypes "github.com/aws-controllers-k8s/sfn-controller/apis/v1alpha1"
)

const testDefinition = `{"StartAt":"Pass","States":{"Pass":{"Type":"Pass","End":true}}}`

func diffPaths(delta *ackcompare.Delta) string {
	paths := make([]string, 0, len(delta.Differences))
	for _, diff := range delta.Differences {
		paths = append(paths, fmt.Sprintf("%v", diff.Path))
	}
	return fmt.Sprintf("%v", paths)
}

func stateMachine(spec svcapitypes.StateMachineSpec) *resource {
	spec.Name = aws.String("test-state-machine")
	spec.RoleARN = aws.String("arn:aws:iam::123456789012:role/test-role")
	spec.Definition = aws.String(testDefinition)
	return &resource{ko: &svcapitypes.StateMachine{Spec: spec}}
}

func awsDefaults() svcapitypes.StateMachineSpec {
	return svcapitypes.StateMachineSpec{
		LoggingConfiguration: &svcapitypes.LoggingConfiguration{
			IncludeExecutionData: aws.Bool(false),
			Level:                aws.String("OFF"),
		},
		TracingConfiguration: &svcapitypes.TracingConfiguration{
			Enabled: aws.Bool(false),
		},
		Type: aws.String("STANDARD"),
	}
}

func TestNewResourceDelta_ServerDefaultsWithoutLateInitialization(t *testing.T) {
	a := stateMachine(svcapitypes.StateMachineSpec{})
	b := stateMachine(awsDefaults())

	delta := newResourceDelta(a, b)

	for _, path := range []string{
		"Spec.LoggingConfiguration",
		"Spec.TracingConfiguration",
		"Spec.Type",
	} {
		if !delta.DifferentAt(path) {
			t.Errorf("expected a difference at %s before late initialization", path)
		}
	}
}

func TestLateInitializeThenNewResourceDelta(t *testing.T) {
	loggingWithDestination := func(includeExecutionData *bool, level *string) *svcapitypes.LoggingConfiguration {
		return &svcapitypes.LoggingConfiguration{
			Destinations: []*svcapitypes.LogDestination{
				{
					CloudWatchLogsLogGroup: &svcapitypes.CloudWatchLogsLogGroup{
						LogGroupARN: aws.String("arn:aws:logs:us-west-2:123456789012:log-group:test:*"),
					},
				},
			},
			IncludeExecutionData: includeExecutionData,
			Level:                level,
		}
	}

	tests := []struct {
		name       string
		declared   svcapitypes.StateMachineSpec
		observed   svcapitypes.StateMachineSpec
		wantDiffAt []string
	}{
		{
			name:     "all three omitted, AWS reports its defaults",
			declared: svcapitypes.StateMachineSpec{},
			observed: awsDefaults(),
		},
		{
			name: "only loggingConfiguration omitted",
			declared: svcapitypes.StateMachineSpec{
				TracingConfiguration: &svcapitypes.TracingConfiguration{Enabled: aws.Bool(true)},
				Type:                 aws.String("EXPRESS"),
			},
			observed: svcapitypes.StateMachineSpec{
				LoggingConfiguration: &svcapitypes.LoggingConfiguration{
					IncludeExecutionData: aws.Bool(false),
					Level:                aws.String("OFF"),
				},
				TracingConfiguration: &svcapitypes.TracingConfiguration{Enabled: aws.Bool(true)},
				Type:                 aws.String("EXPRESS"),
			},
		},
		{
			name: "only tracingConfiguration omitted",
			declared: svcapitypes.StateMachineSpec{
				LoggingConfiguration: loggingWithDestination(aws.Bool(true), aws.String("ALL")),
				Type:                 aws.String("STANDARD"),
			},
			observed: svcapitypes.StateMachineSpec{
				LoggingConfiguration: loggingWithDestination(aws.Bool(true), aws.String("ALL")),
				TracingConfiguration: &svcapitypes.TracingConfiguration{Enabled: aws.Bool(false)},
				Type:                 aws.String("STANDARD"),
			},
		},
		{
			name: "only type omitted",
			declared: svcapitypes.StateMachineSpec{
				LoggingConfiguration: &svcapitypes.LoggingConfiguration{
					IncludeExecutionData: aws.Bool(false),
					Level:                aws.String("OFF"),
				},
				TracingConfiguration: &svcapitypes.TracingConfiguration{Enabled: aws.Bool(false)},
			},
			observed: awsDefaults(),
		},
		{
			name: "loggingConfiguration declared without includeExecutionData",
			declared: svcapitypes.StateMachineSpec{
				LoggingConfiguration: loggingWithDestination(nil, aws.String("ALL")),
			},
			observed: svcapitypes.StateMachineSpec{
				LoggingConfiguration: loggingWithDestination(aws.Bool(false), aws.String("ALL")),
				TracingConfiguration: &svcapitypes.TracingConfiguration{Enabled: aws.Bool(false)},
				Type:                 aws.String("STANDARD"),
			},
		},
		{
			name: "empty tracingConfiguration declared",
			declared: svcapitypes.StateMachineSpec{
				TracingConfiguration: &svcapitypes.TracingConfiguration{},
			},
			observed: awsDefaults(),
		},
		{
			name:     "declared values already match what AWS reports",
			declared: awsDefaults(),
			observed: awsDefaults(),
		},
		{
			name: "declared type differs from what AWS reports",
			declared: svcapitypes.StateMachineSpec{
				Type: aws.String("EXPRESS"),
			},
			observed:   awsDefaults(),
			wantDiffAt: []string{"Spec.Type"},
		},
		{
			name: "declared tracing enabled while AWS reports disabled",
			declared: svcapitypes.StateMachineSpec{
				TracingConfiguration: &svcapitypes.TracingConfiguration{Enabled: aws.Bool(true)},
			},
			observed:   awsDefaults(),
			wantDiffAt: []string{"Spec.TracingConfiguration.Enabled"},
		},
		{
			name: "declared log level differs from what AWS reports",
			declared: svcapitypes.StateMachineSpec{
				LoggingConfiguration: loggingWithDestination(aws.Bool(true), aws.String("ALL")),
			},
			observed:   awsDefaults(),
			wantDiffAt: []string{"Spec.LoggingConfiguration.Level"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rm := &resourceManager{}
			declared := stateMachine(tc.declared)
			observed := stateMachine(tc.observed)

			lateInited := rm.lateInitializeFromReadOneOutput(observed, declared)
			delta := newResourceDelta(lateInited.(*resource), observed)

			if len(tc.wantDiffAt) == 0 {
				if delta.DifferentAt("Spec") {
					t.Errorf("expected no spec difference after late initialization, got %s", diffPaths(delta))
				}
				return
			}
			for _, path := range tc.wantDiffAt {
				if !delta.DifferentAt(path) {
					t.Errorf("expected a difference at %s, got %s", path, diffPaths(delta))
				}
			}
		})
	}
}
