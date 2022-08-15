# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
# 	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for the SFN Activity API.
"""

import pytest
import time
import logging

from acktest import tags
from acktest.resources import random_suffix_name
from acktest.k8s import resource as k8s
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_sfn_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.tests.helper import SFNHelper

RESOURCE_PLURAL = "activities"

CREATE_WAIT_AFTER_SECONDS = 20
UPDATE_WAIT_AFTER_SECONDS = 10
DELETE_WAIT_AFTER_SECONDS = 60

@pytest.fixture
def basic_activity():
    resource_name = random_suffix_name("sfn-activity", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements["ACTIVITY_NAME"] = resource_name

    resource_data = load_sfn_resource(
        "activity",
        additional_replacements=replacements,
    )
    logging.debug(resource_data)

    # Create the k8s resource
    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
        resource_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)

    time.sleep(CREATE_WAIT_AFTER_SECONDS)

    # Get latest activity CR
    cr = k8s.wait_resource_consumed_by_controller(ref)

    assert cr is not None
    assert k8s.get_resource_exists(ref)

    yield (ref, cr)

    # Try to delete, if doesn't already exist
    try:
        _, deleted = k8s.delete_custom_resource(ref, 3, 10)
        assert deleted
    except:
        pass

@service_marker
class TestActivity:
    def test_basic(self, sfn_client, basic_activity):
        (ref, cr) = basic_activity

        activity_arn = cr["status"]["ackResourceMetadata"]["arn"]

        sfn_helper = SFNHelper(sfn_client)
        # verify that activity exists
        assert sfn_helper.activity_exists(activity_arn)

        activity_tags = sfn_helper.get_resource_tags(activity_arn)
        tags.assert_ack_system_tags(
            tags=activity_tags,
            key_member_name = 'key',
            value_member_name  = 'value'
        )
        tags.assert_equal_without_ack_tags(
            actual=cr["spec"]["tags"],
            expected=activity_tags,
            key_member_name = 'key',
            value_member_name  = 'value'
        )

        # updates tags
        # deleting k1 and k2, updating k3 value and adding two new tags
        new_tags = [
            {
                "key": "k3",
                "value": "v3-new",
            },
            {
                "key": "k4",
                "value": "v4",
            },
            {
                "key": "k5",
                "value": "v5",
            }
        ]
        cr["spec"]["tags"] = new_tags
        # Patch k8s resource
        k8s.patch_custom_resource(ref, cr)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        activity_tags = sfn_helper.get_resource_tags(activity_arn)
        tags.assert_equal_without_ack_tags(
            actual=cr["spec"]["tags"],
            expected=activity_tags,
            key_member_name = 'key',
            value_member_name  = 'value'
        )

        # Delete k8s resource
        _, deleted = k8s.delete_custom_resource(ref)
        assert deleted is True

        time.sleep(DELETE_WAIT_AFTER_SECONDS)

        # Check activity doesn't exist
        assert not sfn_helper.activity_exists(activity_arn)