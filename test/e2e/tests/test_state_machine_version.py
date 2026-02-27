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

"""Integration tests for the SFN StateMachineVersion API.
"""

import pytest
import time
import logging

from kubernetes.client.exceptions import ApiException
from acktest.resources import random_suffix_name
from acktest.k8s import resource as k8s
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_sfn_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.tests.helper import SFNHelper
from e2e.bootstrap_resources import get_bootstrap_resources

SM_RESOURCE_PLURAL = "statemachines"
VERSION_RESOURCE_PLURAL = "statemachineversions"

CREATE_WAIT_AFTER_SECONDS = 20
DELETE_WAIT_AFTER_SECONDS = 60


@pytest.fixture
def state_machine_and_version(sfn_client):
    """Create a state machine, then create a version of it."""
    sm_name = random_suffix_name("sfn-sm-ver", 24)

    # Create the state machine first
    sm_replacements = REPLACEMENT_VALUES.copy()
    sm_replacements["STATE_MACHINE_NAME"] = sm_name
    sm_replacements["SFN_EXECUTION_ROLE_ARN"] = get_bootstrap_resources().SfnExecutionRole.arn

    sm_data = load_sfn_resource(
        "state_machine",
        additional_replacements=sm_replacements,
    )
    logging.debug(sm_data)

    sm_ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, SM_RESOURCE_PLURAL,
        sm_name, namespace="default",
    )
    k8s.create_custom_resource(sm_ref, sm_data)
    time.sleep(CREATE_WAIT_AFTER_SECONDS)

    sm_cr = k8s.wait_resource_consumed_by_controller(sm_ref)
    assert sm_cr is not None
    assert k8s.get_resource_exists(sm_ref)

    # Now create a version
    version_name = random_suffix_name("sfn-sm-v1", 24)
    ver_replacements = REPLACEMENT_VALUES.copy()
    ver_replacements["STATE_MACHINE_VERSION_NAME"] = version_name
    ver_replacements["STATE_MACHINE_NAME"] = sm_name
    ver_replacements["VERSION_DESCRIPTION"] = "Version 1 - initial release"

    ver_data = load_sfn_resource(
        "state_machine_version",
        additional_replacements=ver_replacements,
    )
    logging.debug(ver_data)

    ver_ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, VERSION_RESOURCE_PLURAL,
        version_name, namespace="default",
    )
    k8s.create_custom_resource(ver_ref, ver_data)
    time.sleep(CREATE_WAIT_AFTER_SECONDS)

    ver_cr = k8s.wait_resource_consumed_by_controller(ver_ref)
    assert ver_cr is not None
    assert k8s.get_resource_exists(ver_ref)

    yield (sm_ref, sm_cr, ver_ref, ver_cr)

    # Cleanup: delete version first, then state machine
    try:
        _, deleted = k8s.delete_custom_resource(ver_ref, 3, 10)
        assert deleted
    except:
        pass

    time.sleep(DELETE_WAIT_AFTER_SECONDS)

    try:
        _, deleted = k8s.delete_custom_resource(sm_ref, 3, 10)
        assert deleted
    except:
        pass


@service_marker
class TestStateMachineVersion:
    def test_create_and_delete(self, sfn_client, state_machine_and_version):
        (sm_ref, sm_cr, ver_ref, ver_cr) = state_machine_and_version

        sfn_helper = SFNHelper(sfn_client)

        # Verify the version was created and has an ARN
        version_arn = ver_cr["status"]["ackResourceMetadata"]["arn"]
        assert version_arn is not None
        assert ":1" in version_arn  # First version should be :1

        # Verify creation date is set
        assert "creationDate" in ver_cr["status"]

        # Verify the version ARN is also in status
        assert "stateMachineVersionARN" in ver_cr["status"]
        assert ver_cr["status"]["stateMachineVersionARN"] == version_arn

        # Verify the version exists in AWS by describing via the state machine ARN
        version_details = sfn_helper.get_state_machine(version_arn)
        assert version_details is not None

        # Delete the version CR
        _, deleted = k8s.delete_custom_resource(ver_ref)
        assert deleted is True

        time.sleep(DELETE_WAIT_AFTER_SECONDS)

    def test_immutable_update_rejected(self, sfn_client, state_machine_and_version):
        """Verify that updating a version spec causes a rejection.

        Immutable fields are enforced by CRD validation rules
        (x-kubernetes-validations), so the API server rejects the patch
        with a 422 Unprocessable Entity before it reaches the controller.
        """
        (sm_ref, sm_cr, ver_ref, ver_cr) = state_machine_and_version

        # Attempt to update description (should be immutable)
        ver_cr["spec"]["description"] = "Updated description - should fail"
        with pytest.raises(ApiException) as exc_info:
            k8s.patch_custom_resource(ver_ref, ver_cr)

        # The API server should reject the patch with 422
        assert exc_info.value.status == 422
        assert "immutable" in exc_info.value.body.lower()
