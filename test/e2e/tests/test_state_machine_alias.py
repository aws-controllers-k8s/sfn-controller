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

"""Integration tests for the SFN StateMachineAlias API.
"""

import pytest
import time
import logging

from acktest.resources import random_suffix_name
from acktest.k8s import resource as k8s
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_sfn_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.tests.helper import SFNHelper
from e2e.bootstrap_resources import get_bootstrap_resources

SM_RESOURCE_PLURAL = "statemachines"
ALIAS_RESOURCE_PLURAL = "statemachinealiases"

CREATE_WAIT_AFTER_SECONDS = 20
UPDATE_WAIT_AFTER_SECONDS = 10
DELETE_WAIT_AFTER_SECONDS = 60


@pytest.fixture
def state_machine_with_alias(sfn_client):
    """Create a state machine, publish a version via the AWS API, then create an alias."""
    sfn_helper = SFNHelper(sfn_client)
    sm_name = random_suffix_name("sfn-sm-alias", 24)

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
    sm_arn = sm_cr["status"]["ackResourceMetadata"]["arn"]

    # Publish a version directly via AWS API (for simplicity)
    version_resp = sfn_helper.publish_state_machine_version(sm_arn, "Version for alias test")
    assert version_resp is not None
    version_arn = version_resp["stateMachineVersionArn"]

    # Create alias pointing to the version
    alias_name = random_suffix_name("sfn-alias", 24)
    alias_logical_name = "production"

    alias_replacements = REPLACEMENT_VALUES.copy()
    alias_replacements["STATE_MACHINE_ALIAS_NAME"] = alias_name
    alias_replacements["ALIAS_NAME"] = alias_logical_name
    alias_replacements["ALIAS_DESCRIPTION"] = "Production traffic"
    alias_replacements["VERSION_ARN"] = version_arn

    alias_data = load_sfn_resource(
        "state_machine_alias",
        additional_replacements=alias_replacements,
    )
    logging.debug(alias_data)

    alias_ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, ALIAS_RESOURCE_PLURAL,
        alias_name, namespace="default",
    )
    k8s.create_custom_resource(alias_ref, alias_data)
    time.sleep(CREATE_WAIT_AFTER_SECONDS)

    alias_cr = k8s.wait_resource_consumed_by_controller(alias_ref)
    assert alias_cr is not None
    assert k8s.get_resource_exists(alias_ref)

    yield (sm_ref, sm_cr, alias_ref, alias_cr, version_arn)

    # Cleanup: delete alias first, then version, then state machine
    try:
        _, deleted = k8s.delete_custom_resource(alias_ref, 3, 10)
        assert deleted
    except:
        pass

    time.sleep(DELETE_WAIT_AFTER_SECONDS)

    try:
        sfn_helper.delete_state_machine_version(version_arn)
    except:
        pass

    try:
        _, deleted = k8s.delete_custom_resource(sm_ref, 3, 10)
        assert deleted
    except:
        pass


@service_marker
class TestStateMachineAlias:
    def test_create_and_delete(self, sfn_client, state_machine_with_alias):
        (sm_ref, sm_cr, alias_ref, alias_cr, version_arn) = state_machine_with_alias

        sfn_helper = SFNHelper(sfn_client)

        # Verify alias was created and has an ARN
        alias_arn = alias_cr["status"]["ackResourceMetadata"]["arn"]
        assert alias_arn is not None

        # Verify creation date is set
        assert "creationDate" in alias_cr["status"]

        # Verify alias exists in AWS
        alias_details = sfn_helper.describe_state_machine_alias(alias_arn)
        assert alias_details is not None
        assert alias_details["name"] == "production"
        assert alias_details["description"] == "Production traffic"
        assert len(alias_details["routingConfiguration"]) == 1
        assert alias_details["routingConfiguration"][0]["stateMachineVersionArn"] == version_arn
        assert alias_details["routingConfiguration"][0]["weight"] == 100

        # Delete the alias CR
        _, deleted = k8s.delete_custom_resource(alias_ref)
        assert deleted is True

        time.sleep(DELETE_WAIT_AFTER_SECONDS)

        # Verify alias no longer exists in AWS
        assert not sfn_helper.state_machine_alias_exists(alias_arn)

    def test_update_routing_configuration(self, sfn_client, state_machine_with_alias):
        """Test updating the routing configuration of an alias."""
        (sm_ref, sm_cr, alias_ref, alias_cr, version_arn) = state_machine_with_alias

        sfn_helper = SFNHelper(sfn_client)
        alias_arn = alias_cr["status"]["ackResourceMetadata"]["arn"]

        # Update the description
        alias_cr["spec"]["description"] = "Updated production traffic"
        k8s.patch_custom_resource(alias_ref, alias_cr)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        # Verify the update in AWS
        alias_details = sfn_helper.describe_state_machine_alias(alias_arn)
        assert alias_details is not None
        assert alias_details["description"] == "Updated production traffic"
