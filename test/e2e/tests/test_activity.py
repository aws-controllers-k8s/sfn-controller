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
from typing import Generator
from dataclasses import dataclass

from acktest.resources import random_suffix_name
from acktest.k8s import resource as k8s
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_sfn_resource
from e2e.replacement_values import REPLACEMENT_VALUES

RESOURCE_PLURAL = "activities"

CREATE_WAIT_AFTER_SECONDS = 20
DELETE_WAIT_AFTER_SECONDS = 60


@dataclass
class Activity:
    ref: k8s.CustomResourceReference
    resource_name: str
    resource_data: str
    arn: str


def activity_exists(sfn_client, activity: Activity) -> bool:
    try:
        resp = sfn_client.describe_activity(activityArn=activity.arn)
    except Exception as e:
        logging.debug(e)
        return False

    if resp["name"] == activity.resource_name:
        return True

    return False


def load_activity_resource(resource_file_name: str, resource_name: str):
    replacements = REPLACEMENT_VALUES.copy()
    replacements["ACTIVITY_NAME"] = resource_name

    resource_data = load_sfn_resource(
        resource_file_name,
        additional_replacements=replacements,
    )
    logging.debug(resource_data)
    return resource_data


def create_activity(resource_file_name: str) -> Activity:
    resource_name = random_suffix_name("sfn-activity", 24)
    resource_data = load_activity_resource(resource_file_name, resource_name)

    logging.info(f"Creating Activity {resource_name}")
    # Create k8s resource
    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
        resource_name, namespace="default",
    )
    resource_data = k8s.create_custom_resource(ref, resource_data)
    k8s.wait_resource_consumed_by_controller(ref)

    time.sleep(CREATE_WAIT_AFTER_SECONDS)

    res = k8s.get_resource(ref)
    arn = k8s.get_resource_arn(res)

    return Activity(ref, resource_name, resource_data, arn)


def delete_activity(activity: Activity):
    # Delete k8s resource
    _, deleted = k8s.delete_custom_resource(activity.ref)
    assert deleted is True

    time.sleep(DELETE_WAIT_AFTER_SECONDS)

@pytest.fixture(scope="function")
def basic_activity(sfn_client) -> Generator[Activity, None, None]:
    activity = None
    try:
        activity = create_activity("activity")
        assert k8s.get_resource_exists(activity.ref)

        exists = activity_exists(sfn_client, activity)
        assert exists
    except:
        if activity is not None:
            delete_activity(activity)
        return pytest.fail("Activity failed to create")

    yield activity

    exists = activity_exists(sfn_client, activity)
    if exists:
        delete_activity(activity)


@service_marker
class TestActivity:
    def test_basic(self, basic_activity, sfn_client):
        # Existance assertions are handled by the fixture
        assert basic_activity

        delete_activity(basic_activity)
        exists = activity_exists(sfn_client, basic_activity)
        assert not exists
