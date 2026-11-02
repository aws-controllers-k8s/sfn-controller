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
	"reflect"
	"testing"

	svcsdk "github.com/aws/aws-sdk-go-v2/service/sfn"

	svcapitypes "github.com/aws-controllers-k8s/sfn-controller/apis/v1alpha1"
)

type fakeTagsClient struct {
	tagCalls   []*svcsdk.TagResourceInput
	untagCalls []*svcsdk.UntagResourceInput
}

func (f *fakeTagsClient) TagResource(
	_ context.Context,
	in *svcsdk.TagResourceInput,
	_ ...func(*svcsdk.Options),
) (*svcsdk.TagResourceOutput, error) {
	f.tagCalls = append(f.tagCalls, in)
	return &svcsdk.TagResourceOutput{}, nil
}

func (f *fakeTagsClient) ListTagsForResource(
	_ context.Context,
	_ *svcsdk.ListTagsForResourceInput,
	_ ...func(*svcsdk.Options),
) (*svcsdk.ListTagsForResourceOutput, error) {
	return &svcsdk.ListTagsForResourceOutput{}, nil
}

func (f *fakeTagsClient) UntagResource(
	_ context.Context,
	in *svcsdk.UntagResourceInput,
	_ ...func(*svcsdk.Options),
) (*svcsdk.UntagResourceOutput, error) {
	f.untagCalls = append(f.untagCalls, in)
	return &svcsdk.UntagResourceOutput{}, nil
}

type noopMetrics struct{}

func (noopMetrics) RecordAPICall(string, string, error) {}

func strPtr(s string) *string {
	return &s
}

func tag(key, value string) *svcapitypes.Tag {
	return &svcapitypes.Tag{Key: strPtr(key), Value: strPtr(value)}
}

func TestSyncResourceTags_NoChange(t *testing.T) {
	client := &fakeTagsClient{}
	latest := []*svcapitypes.Tag{tag("a", "1"), tag("b", "2")}
	desired := []*svcapitypes.Tag{tag("b", "2"), tag("a", "1")}

	if err := SyncResourceTags(context.Background(), client, noopMetrics{}, "arn", latest, desired); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.tagCalls) != 0 || len(client.untagCalls) != 0 {
		t.Errorf("expected no API calls, got tag=%d untag=%d", len(client.tagCalls), len(client.untagCalls))
	}
}

func TestSyncResourceTags_Add(t *testing.T) {
	client := &fakeTagsClient{}
	latest := []*svcapitypes.Tag{tag("a", "1")}
	desired := []*svcapitypes.Tag{tag("a", "1"), tag("c", "3"), tag("b", "2")}

	if err := SyncResourceTags(context.Background(), client, noopMetrics{}, "arn", latest, desired); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.untagCalls) != 0 {
		t.Errorf("expected no UntagResource calls, got %d", len(client.untagCalls))
	}
	if len(client.tagCalls) != 1 {
		t.Fatalf("expected 1 TagResource call, got %d", len(client.tagCalls))
	}
	got := client.tagCalls[0].Tags
	want := sdkTagsFromResourceTags([]*svcapitypes.Tag{tag("b", "2"), tag("c", "3")})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("TagResource tags = %+v, want %+v", got, want)
	}
}

func TestSyncResourceTags_Remove(t *testing.T) {
	client := &fakeTagsClient{}
	latest := []*svcapitypes.Tag{tag("a", "1"), tag("b", "2"), tag("c", "3")}
	desired := []*svcapitypes.Tag{tag("b", "2")}

	if err := SyncResourceTags(context.Background(), client, noopMetrics{}, "arn", latest, desired); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.tagCalls) != 0 {
		t.Errorf("expected no TagResource calls, got %d", len(client.tagCalls))
	}
	if len(client.untagCalls) != 1 {
		t.Fatalf("expected 1 UntagResource call, got %d", len(client.untagCalls))
	}
	want := []string{"a", "c"}
	if !reflect.DeepEqual(client.untagCalls[0].TagKeys, want) {
		t.Errorf("UntagResource keys = %v, want %v", client.untagCalls[0].TagKeys, want)
	}
}

func TestSyncResourceTags_ValueChange(t *testing.T) {
	client := &fakeTagsClient{}
	latest := []*svcapitypes.Tag{tag("a", "1")}
	desired := []*svcapitypes.Tag{tag("a", "changed")}

	if err := SyncResourceTags(context.Background(), client, noopMetrics{}, "arn", latest, desired); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.untagCalls) != 0 {
		t.Errorf("expected no UntagResource calls, got %d", len(client.untagCalls))
	}
	if len(client.tagCalls) != 1 {
		t.Fatalf("expected 1 TagResource call, got %d", len(client.tagCalls))
	}
	want := sdkTagsFromResourceTags([]*svcapitypes.Tag{tag("a", "changed")})
	if !reflect.DeepEqual(client.tagCalls[0].Tags, want) {
		t.Errorf("TagResource tags = %+v, want %+v", client.tagCalls[0].Tags, want)
	}
}

func TestSyncResourceTags_NilKeyNoPanic(t *testing.T) {
	client := &fakeTagsClient{}
	latest := []*svcapitypes.Tag{nil, {Value: strPtr("orphan")}, tag("a", "1")}
	desired := []*svcapitypes.Tag{{Value: strPtr("orphan")}, nil, tag("a", "1")}

	if err := SyncResourceTags(context.Background(), client, noopMetrics{}, "arn", latest, desired); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.tagCalls) != 0 || len(client.untagCalls) != 0 {
		t.Errorf("expected no API calls, got tag=%d untag=%d", len(client.tagCalls), len(client.untagCalls))
	}
}

func TestSyncResourceTags_AddAndRemove(t *testing.T) {
	client := &fakeTagsClient{}
	latest := []*svcapitypes.Tag{tag("a", "1"), tag("b", "2")}
	desired := []*svcapitypes.Tag{tag("b", "2"), tag("c", "3")}
	err := SyncResourceTags(context.Background(), client, noopMetrics{}, "arn", latest, desired)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(client.untagCalls) != 1 {
		t.Fatalf("UntagResource calls = %d, want 1", len(client.untagCalls))
	}
	if !reflect.DeepEqual(client.untagCalls[0].TagKeys, []string{"a"}) {
		t.Errorf("UntagResource keys = %v, want [a]", client.untagCalls[0].TagKeys)
	}
	if len(client.tagCalls) != 1 {
		t.Fatalf("TagResource calls = %d, want 1", len(client.tagCalls))
	}
	wantTags := sdkTagsFromResourceTags([]*svcapitypes.Tag{tag("c", "3")})
	if !reflect.DeepEqual(client.tagCalls[0].Tags, wantTags) {
		t.Errorf("TagResource tags = %v, want %v", client.tagCalls[0].Tags, wantTags)
	}
}
