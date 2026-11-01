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

package util

import (
	"context"
	"maps"
	"slices"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	acktags "github.com/aws-controllers-k8s/runtime/pkg/tags"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/sfn"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/sfn/types"

	svcapitypes "github.com/aws-controllers-k8s/sfn-controller/apis/v1alpha1"
)

// TODO(a-hilaly) most of the utility in this package should ideally go to
// ack runtime repository.

type metricsRecorder interface {
	RecordAPICall(opType string, opID string, err error)
}

type tagsClient interface {
	TagResource(context.Context, *svcsdk.TagResourceInput, ...func(*svcsdk.Options)) (*svcsdk.TagResourceOutput, error)
	ListTagsForResource(context.Context, *svcsdk.ListTagsForResourceInput, ...func(*svcsdk.Options)) (*svcsdk.ListTagsForResourceOutput, error)
	UntagResource(context.Context, *svcsdk.UntagResourceInput, ...func(*svcsdk.Options)) (*svcsdk.UntagResourceOutput, error)
}

// GetResourceTags retrieves a resource list of tags.
func GetResourceTags(
	ctx context.Context,
	client tagsClient,
	mr metricsRecorder,
	resourceARN string,
) ([]*svcapitypes.Tag, error) {
	listTagsForResourceResponse, err := client.ListTagsForResource(
		ctx,
		&svcsdk.ListTagsForResourceInput{
			ResourceArn: &resourceARN,
		},
	)
	mr.RecordAPICall("GET", "ListTagsForResource", err)
	if err != nil {
		return nil, err
	}
	tags := make([]*svcapitypes.Tag, 0, len(listTagsForResourceResponse.Tags))
	for _, tag := range listTagsForResourceResponse.Tags {
		tags = append(tags, &svcapitypes.Tag{
			Key:   tag.Key,
			Value: tag.Value,
		})
	}
	return tags, nil
}

// SyncResourceTags uses TagResource and UntagResource API Calls to add, remove
// and update resource tags.
func SyncResourceTags(
	ctx context.Context,
	client tagsClient,
	mr metricsRecorder,
	resourceARN string,
	latestTags []*svcapitypes.Tag,
	desiredTags []*svcapitypes.Tag,
) error {
	var err error
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("common.SyncResourceTags")
	defer func() {
		exit(err)
	}()

	added, _, removed := ackcompare.GetTagsDifference(
		toACKTags(latestTags),
		toACKTags(desiredTags),
	)
	if len(added) == 0 && len(removed) == 0 {
		return nil
	}

	if len(removed) > 0 {
		_, err = client.UntagResource(
			ctx,
			&svcsdk.UntagResourceInput{
				ResourceArn: aws.String(resourceARN),
				TagKeys:     slices.Sorted(maps.Keys(removed)),
			},
		)
		mr.RecordAPICall("UPDATE", "UntagResource", err)
		if err != nil {
			return err
		}
	}

	if len(added) > 0 {
		addedTags := make([]*svcapitypes.Tag, 0, len(added))
		for _, k := range slices.Sorted(maps.Keys(added)) {
			addedTags = append(addedTags, &svcapitypes.Tag{
				Key:   aws.String(k),
				Value: aws.String(added[k]),
			})
		}
		_, err = client.TagResource(
			ctx,
			&svcsdk.TagResourceInput{
				ResourceArn: aws.String(resourceARN),
				Tags:        sdkTagsFromResourceTags(addedTags),
			},
		)
		mr.RecordAPICall("UPDATE", "TagResource", err)
		if err != nil {
			return err
		}
	}
	return nil
}

// toACKTags transforms a *svcapitypes.Tag array to an ACK acktags.Tags map.
// It is nil-safe: nil elements and tags without a key are skipped, and a nil
// value is stored as an empty string. The per-resource, code-generated
// convertToOrderedACKTags (pkg/resource/*/tags.go) is the counterpart used in
// hook code; unlike this function it does not guard against nil elements.
func toACKTags(tags []*svcapitypes.Tag) acktags.Tags {
	m := make(acktags.Tags)
	for _, t := range tags {
		if t == nil || t.Key == nil {
			continue
		}
		v := ""
		if t.Value != nil {
			v = *t.Value
		}
		m[*t.Key] = v
	}
	return m
}

// svcTagsFromResourceTags transforms a *svcapitypes.Tag array to a *svcsdk.Tag array.
func sdkTagsFromResourceTags(rTags []*svcapitypes.Tag) []svcsdktypes.Tag {
	tags := make([]svcsdktypes.Tag, len(rTags))
	for i := range rTags {
		tags[i] = svcsdktypes.Tag{
			Key:   rTags[i].Key,
			Value: rTags[i].Value,
		}
	}
	return tags
}
