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

package activity

import (
	"context"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"

	svcapitypes "github.com/aws-controllers-k8s/sfn-controller/apis/v1alpha1"
	commonutil "github.com/aws-controllers-k8s/sfn-controller/pkg/util"
)

// setResourceAdditionalFields queries and adds the tags to an Activity resource
func (rm *resourceManager) setResourceAdditionalFields(
	ctx context.Context,
	ko *svcapitypes.Activity,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.setResourceAdditionalFields")
	defer func() {
		exit(err)
	}()

	// Set activity tags
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

// customUpdateActivity patches each of the resource properties in the backend AWS
// service API and returns a new resource with updated fields.
func (rm *resourceManager) customUpdateActivity(
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
