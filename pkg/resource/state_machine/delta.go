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

// Code generated by ack-generate. DO NOT EDIT.

package state_machine

import (
	"bytes"
	"reflect"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	acktags "github.com/aws-controllers-k8s/runtime/pkg/tags"
)

// Hack to avoid import errors during build...
var (
	_ = &bytes.Buffer{}
	_ = &reflect.Method{}
	_ = &acktags.Tags{}
)

// newResourceDelta returns a new `ackcompare.Delta` used to compare two
// resources
func newResourceDelta(
	a *resource,
	b *resource,
) *ackcompare.Delta {
	delta := ackcompare.NewDelta()
	if (a == nil && b != nil) ||
		(a != nil && b == nil) {
		delta.Add("", a, b)
		return delta
	}
	customPreCompare(delta, a, b)

	if ackcompare.HasNilDifference(a.ko.Spec.Definition, b.ko.Spec.Definition) {
		delta.Add("Spec.Definition", a.ko.Spec.Definition, b.ko.Spec.Definition)
	} else if a.ko.Spec.Definition != nil && b.ko.Spec.Definition != nil {
		if *a.ko.Spec.Definition != *b.ko.Spec.Definition {
			delta.Add("Spec.Definition", a.ko.Spec.Definition, b.ko.Spec.Definition)
		}
	}
	if ackcompare.HasNilDifference(a.ko.Spec.LoggingConfiguration, b.ko.Spec.LoggingConfiguration) {
		delta.Add("Spec.LoggingConfiguration", a.ko.Spec.LoggingConfiguration, b.ko.Spec.LoggingConfiguration)
	} else if a.ko.Spec.LoggingConfiguration != nil && b.ko.Spec.LoggingConfiguration != nil {
		if len(a.ko.Spec.LoggingConfiguration.Destinations) != len(b.ko.Spec.LoggingConfiguration.Destinations) {
			delta.Add("Spec.LoggingConfiguration.Destinations", a.ko.Spec.LoggingConfiguration.Destinations, b.ko.Spec.LoggingConfiguration.Destinations)
		} else if len(a.ko.Spec.LoggingConfiguration.Destinations) > 0 {
			if !reflect.DeepEqual(a.ko.Spec.LoggingConfiguration.Destinations, b.ko.Spec.LoggingConfiguration.Destinations) {
				delta.Add("Spec.LoggingConfiguration.Destinations", a.ko.Spec.LoggingConfiguration.Destinations, b.ko.Spec.LoggingConfiguration.Destinations)
			}
		}
		if ackcompare.HasNilDifference(a.ko.Spec.LoggingConfiguration.IncludeExecutionData, b.ko.Spec.LoggingConfiguration.IncludeExecutionData) {
			delta.Add("Spec.LoggingConfiguration.IncludeExecutionData", a.ko.Spec.LoggingConfiguration.IncludeExecutionData, b.ko.Spec.LoggingConfiguration.IncludeExecutionData)
		} else if a.ko.Spec.LoggingConfiguration.IncludeExecutionData != nil && b.ko.Spec.LoggingConfiguration.IncludeExecutionData != nil {
			if *a.ko.Spec.LoggingConfiguration.IncludeExecutionData != *b.ko.Spec.LoggingConfiguration.IncludeExecutionData {
				delta.Add("Spec.LoggingConfiguration.IncludeExecutionData", a.ko.Spec.LoggingConfiguration.IncludeExecutionData, b.ko.Spec.LoggingConfiguration.IncludeExecutionData)
			}
		}
		if ackcompare.HasNilDifference(a.ko.Spec.LoggingConfiguration.Level, b.ko.Spec.LoggingConfiguration.Level) {
			delta.Add("Spec.LoggingConfiguration.Level", a.ko.Spec.LoggingConfiguration.Level, b.ko.Spec.LoggingConfiguration.Level)
		} else if a.ko.Spec.LoggingConfiguration.Level != nil && b.ko.Spec.LoggingConfiguration.Level != nil {
			if *a.ko.Spec.LoggingConfiguration.Level != *b.ko.Spec.LoggingConfiguration.Level {
				delta.Add("Spec.LoggingConfiguration.Level", a.ko.Spec.LoggingConfiguration.Level, b.ko.Spec.LoggingConfiguration.Level)
			}
		}
	}
	if ackcompare.HasNilDifference(a.ko.Spec.Name, b.ko.Spec.Name) {
		delta.Add("Spec.Name", a.ko.Spec.Name, b.ko.Spec.Name)
	} else if a.ko.Spec.Name != nil && b.ko.Spec.Name != nil {
		if *a.ko.Spec.Name != *b.ko.Spec.Name {
			delta.Add("Spec.Name", a.ko.Spec.Name, b.ko.Spec.Name)
		}
	}
	if ackcompare.HasNilDifference(a.ko.Spec.RoleARN, b.ko.Spec.RoleARN) {
		delta.Add("Spec.RoleARN", a.ko.Spec.RoleARN, b.ko.Spec.RoleARN)
	} else if a.ko.Spec.RoleARN != nil && b.ko.Spec.RoleARN != nil {
		if *a.ko.Spec.RoleARN != *b.ko.Spec.RoleARN {
			delta.Add("Spec.RoleARN", a.ko.Spec.RoleARN, b.ko.Spec.RoleARN)
		}
	}
	if ackcompare.HasNilDifference(a.ko.Spec.TracingConfiguration, b.ko.Spec.TracingConfiguration) {
		delta.Add("Spec.TracingConfiguration", a.ko.Spec.TracingConfiguration, b.ko.Spec.TracingConfiguration)
	} else if a.ko.Spec.TracingConfiguration != nil && b.ko.Spec.TracingConfiguration != nil {
		if ackcompare.HasNilDifference(a.ko.Spec.TracingConfiguration.Enabled, b.ko.Spec.TracingConfiguration.Enabled) {
			delta.Add("Spec.TracingConfiguration.Enabled", a.ko.Spec.TracingConfiguration.Enabled, b.ko.Spec.TracingConfiguration.Enabled)
		} else if a.ko.Spec.TracingConfiguration.Enabled != nil && b.ko.Spec.TracingConfiguration.Enabled != nil {
			if *a.ko.Spec.TracingConfiguration.Enabled != *b.ko.Spec.TracingConfiguration.Enabled {
				delta.Add("Spec.TracingConfiguration.Enabled", a.ko.Spec.TracingConfiguration.Enabled, b.ko.Spec.TracingConfiguration.Enabled)
			}
		}
	}
	if ackcompare.HasNilDifference(a.ko.Spec.Type, b.ko.Spec.Type) {
		delta.Add("Spec.Type", a.ko.Spec.Type, b.ko.Spec.Type)
	} else if a.ko.Spec.Type != nil && b.ko.Spec.Type != nil {
		if *a.ko.Spec.Type != *b.ko.Spec.Type {
			delta.Add("Spec.Type", a.ko.Spec.Type, b.ko.Spec.Type)
		}
	}

	return delta
}
