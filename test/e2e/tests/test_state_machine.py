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

"""Integration tests for the SFN StateMachine API.
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

RESOURCE_PLURAL = "statemachines"

CREATE_WAIT_AFTER_SECONDS = 20
DELETE_WAIT_AFTER_SECONDS = 60


@dataclass
class StateMachine:
    ref: k8s.CustomResourceReference
    resource_name: str
    resource_data: str
    arn: str


def state_machine_exists(sfn_client, state_machine: StateMachine) -> bool:
    try:
        resp = sfn_client.describe_state_machine(stateMachineArn=state_machine.arn)
    except Exception as e:
        logging.debug(e)
        return False

    if resp["name"] == state_machine.resource_name:
        return True

    return False


def load_state_machine_resource(resource_file_name: str, resource_name: str):
    replacements = REPLACEMENT_VALUES.copy()
    replacements["STATE_MACHINE_NAME"] = resource_name

    resource_data = load_sfn_resource(
        resource_file_name,
        additional_replacements=replacements,
    )
    logging.debug(resource_data)
    return resource_data


def create_state_machine(resource_file_name: str) -> StateMachine:
    resource_name = random_suffix_name("sfn-state-machine", 24)
    resource_data = load_state_machine_resource(resource_file_name, resource_name)

    logging.info(f"Creating StateMachine {resource_name}")
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

    return StateMachine(ref, resource_name, resource_data, arn)


def delete_state_machine(state_machine: StateMachine):
    # Delete k8s resource
    _, deleted = k8s.delete_custom_resource(state_machine.ref)
    assert deleted is True

    time.sleep(DELETE_WAIT_AFTER_SECONDS)


@pytest.fixture(scope="function")
def basic_state_machine(sfn_client) -> Generator[StateMachine, None, None]:
    state_machine = None
    try:
        state_machine = create_state_machine("state_machine")
        assert k8s.get_resource_exists(state_machine.ref)

        exists = state_machine_exists(sfn_client, state_machine)
        assert exists
    except:
        if state_machine is not None:
            delete_state_machine(state_machine)
        return pytest.fail("StateMachine failed to create")

    yield state_machine

    exists = state_machine_exists(sfn_client, state_machine)
    if exists:
        delete_state_machine(state_machine)


@service_marker
class TestStateMachine:
    def test_basic(self, basic_state_machine, sfn_client):
        # Existance assertions are handled by the fixture
        assert basic_state_machine

        delete_state_machine(basic_state_machine)
        exists = state_machine_exists(sfn_client, basic_state_machine)
        assert not exists
