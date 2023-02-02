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

	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	ackutil "github.com/aws-controllers-k8s/runtime/pkg/util"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/request"
	svcsdk "github.com/aws/aws-sdk-go/service/sfn"

	svcapitypes "github.com/aws-controllers-k8s/sfn-controller/apis/v1alpha1"
)

// TODO(a-hilaly) most of the utility in this package should ideally go to
// ack runtime repository.

type metricsRecorder interface {
	RecordAPICall(opType string, opID string, err error)
}

type tagsClient interface {
	TagResourceWithContext(context.Context, *svcsdk.TagResourceInput, ...request.Option) (*svcsdk.TagResourceOutput, error)
	ListTagsForResourceWithContext(context.Context, *svcsdk.ListTagsForResourceInput, ...request.Option) (*svcsdk.ListTagsForResourceOutput, error)
	UntagResourceWithContext(context.Context, *svcsdk.UntagResourceInput, ...request.Option) (*svcsdk.UntagResourceOutput, error)
}

// GetResourceTags retrieves a resource list of tags.
func GetResourceTags(
	ctx context.Context,
	client tagsClient,
	mr metricsRecorder,
	resourceARN string,
) ([]*svcapitypes.Tag, error) {
	listTagsForResourceResponse, err := client.ListTagsForResourceWithContext(
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

	addedOrUpdated, removed := computeTagsDelta(latestTags, desiredTags)

	if len(removed) > 0 {
		_, err = client.UntagResourceWithContext(
			ctx,
			&svcsdk.UntagResourceInput{
				ResourceArn: aws.String(resourceARN),
				TagKeys:     removed,
			},
		)
		mr.RecordAPICall("UPDATE", "UntagResource", err)
		if err != nil {
			return err
		}
	}

	if len(addedOrUpdated) > 0 {
		_, err = client.TagResourceWithContext(
			ctx,
			&svcsdk.TagResourceInput{
				ResourceArn: aws.String(resourceARN),
				Tags:        sdkTagsFromResourceTags(addedOrUpdated),
			},
		)
		mr.RecordAPICall("UPDATE", "TagResource", err)
		if err != nil {
			return err
		}
	}
	return nil
}

// computeTagsDelta compares two Tag arrays and return two different list
// containing the addedOrupdated and removed tags. The removed tags array
// only contains the tags Keys.
func computeTagsDelta(
	a []*svcapitypes.Tag,
	b []*svcapitypes.Tag,
) (addedOrUpdated []*svcapitypes.Tag, removed []*string) {
	var visitedIndexes []string
mainLoop:
	for _, aElement := range a {
		visitedIndexes = append(visitedIndexes, *aElement.Key)
		for _, bElement := range b {
			if equalStrings(aElement.Key, bElement.Key) {
				if !equalStrings(aElement.Value, bElement.Value) {
					addedOrUpdated = append(addedOrUpdated, bElement)
				}
				continue mainLoop
			}
		}
		removed = append(removed, aElement.Key)
	}
	for _, bElement := range b {
		if !ackutil.InStrings(*bElement.Key, visitedIndexes) {
			addedOrUpdated = append(addedOrUpdated, bElement)
		}
	}
	return addedOrUpdated, removed
}

// equalTags returns true if two Tag arrays are equal regardless of the order
// of their elements.
func EqualTags(
	a []*svcapitypes.Tag,
	b []*svcapitypes.Tag,
) bool {
	addedOrUpdated, removed := computeTagsDelta(a, b)
	return len(addedOrUpdated) == 0 && len(removed) == 0
}

// svcTagsFromResourceTags transforms a *svcapitypes.Tag array to a *svcsdk.Tag array.
func sdkTagsFromResourceTags(rTags []*svcapitypes.Tag) []*svcsdk.Tag {
	tags := make([]*svcsdk.Tag, len(rTags))
	for i := range rTags {
		tags[i] = &svcsdk.Tag{
			Key:   rTags[i].Key,
			Value: rTags[i].Value,
		}
	}
	return tags
}

func equalStrings(a, b *string) bool {
	if a == nil {
		return b == nil || *b == ""
	}
	return (*a == "" && b == nil) || *a == *b
}
