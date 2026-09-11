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

import itertools
import pytest
import time
import logging

from acktest import tags
from acktest.resources import random_suffix_name
from acktest.k8s import resource as k8s
from acktest.k8s import condition
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_sfn_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e.tests.helper import SFNHelper
from e2e.bootstrap_resources import get_bootstrap_resources

RESOURCE_PLURAL = "statemachines"

CREATE_WAIT_AFTER_SECONDS = 20
UPDATE_WAIT_AFTER_SECONDS = 10
DELETE_WAIT_AFTER_SECONDS = 60

# Publishing settles a step behind the update that triggers it, so the version
# assertions allow more room than the tag and definition checks.
SETTLE_WAIT_AFTER_SECONDS = 20

# Polling budget for changes the controller makes on its own, after the CR has
# already been patched or annotated.
RECONCILE_POLL_SECONDS = 5
RECONCILE_POLL_ATTEMPTS = 24

UPDATED_DEFINITION = (
    '{"StartAt":"HelloWorld","States":{"HelloWorld":'
    '{"Type":"Pass","Result":"Updated!","End":true}}}'
)

# Makes each _force_reconcile patch a distinct value, so consecutive nudges
# cannot collide on the same second and be dropped as a no-op patch.
_reconcile_nudge = itertools.count(1)

@pytest.fixture
def basic_state_machine():
    resource_name = random_suffix_name("sfn-statemachine", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements["STATE_MACHINE_NAME"] = resource_name
    replacements["SFN_EXECUTION_ROLE_ARN"] = get_bootstrap_resources().SfnExecutionRole.arn

    resource_data = load_sfn_resource(
        "state_machine",
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

    # Get latest state machine CR
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

def _create_state_machine_for_publish(publish: bool):
    """Create a StateMachine CR with spec.publish explicitly set."""
    resource_name = random_suffix_name("sfn-publish", 24)
    replacements = REPLACEMENT_VALUES.copy()
    replacements["STATE_MACHINE_NAME"] = resource_name
    replacements["SFN_EXECUTION_ROLE_ARN"] = get_bootstrap_resources().SfnExecutionRole.arn
    replacements["PUBLISH"] = "true" if publish else "false"

    resource_data = load_sfn_resource(
        "state_machine_publish",
        additional_replacements=replacements,
    )
    logging.debug(resource_data)

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, RESOURCE_PLURAL,
        resource_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    time.sleep(CREATE_WAIT_AFTER_SECONDS)

    cr = k8s.wait_resource_consumed_by_controller(ref)
    assert cr is not None
    assert k8s.wait_on_condition(
        ref, condition.CONDITION_TYPE_RESOURCE_SYNCED, "True", wait_periods=5
    )
    return ref, k8s.get_resource(ref)


def _delete_state_machine(ref):
    """Delete the CR, tolerating a test that already deleted it.

    A failed delete is allowed to raise: swallowing it hides leaked state
    machines behind a green run.
    """
    if not k8s.get_resource_exists(ref):
        return
    _, deleted = k8s.delete_custom_resource(ref, 3, 10)
    assert deleted


def _wait_for(predicate, description: str):
    """Poll a predicate, returning its value once truthy.

    Used for state the controller reaches by itself. Fails the test rather than
    hanging so a regression reports as an assertion instead of a timeout.
    """
    for _ in range(RECONCILE_POLL_ATTEMPTS):
        value = predicate()
        if value:
            return value
        time.sleep(RECONCILE_POLL_SECONDS)
    pytest.fail(
        f"timed out after "
        f"{RECONCILE_POLL_ATTEMPTS * RECONCILE_POLL_SECONDS}s waiting for {description}"
    )


def _force_reconcile(ref):
    """Nudge the controller into reading the resource again.

    The default resync period is 10 hours, so a change made only in AWS is not
    otherwise observed within a test's lifetime.

    A spec change is required. ACK filters watch events with
    GenerationChangedPredicate unless the ignore-field-drift feature gate is
    enabled, and annotations do not bump metadata.generation, so an annotation
    patch is simply never delivered.

    Tags are the one spec field that cannot publish a version by itself:
    customUpdateStateMachine syncs them and then short-circuits on
    DifferentExcept("Spec.Tags") before reaching UpdateStateMachine, so a
    tags-only delta never carries Publish. Any version appearing after this call
    therefore came from the publish path under test, which is what makes the
    "nothing further was published" assertions meaningful.
    """
    k8s.patch_custom_resource(
        ref,
        {
            "spec": {
                "tags": [
                    {
                        "key": "e2e-force-reconcile",
                        "value": f"{int(time.time())}-{next(_reconcile_nudge)}",
                    }
                ]
            }
        },
    )


def _version_arn(ref):
    return k8s.get_resource(ref)["status"].get("stateMachineVersionARN")


@pytest.fixture
def state_machine_published():
    """A state machine created with spec.publish already true."""
    ref, cr = _create_state_machine_for_publish(publish=True)
    yield ref, cr
    _delete_state_machine(ref)


@pytest.fixture
def state_machine_unpublished():
    """A state machine created with spec.publish false, so it has no versions."""
    ref, cr = _create_state_machine_for_publish(publish=False)
    yield ref, cr
    _delete_state_machine(ref)


@service_marker
class TestStateMachine:
    def test_basic(self, sfn_client, basic_state_machine):
        (ref, cr) = basic_state_machine

        state_machine_arn = cr["status"]["ackResourceMetadata"]["arn"]

        sfn_helper = SFNHelper(sfn_client)
        # verify that state machine exists
        assert sfn_helper.state_machine_exists(state_machine_arn)

        state_machine_tags = sfn_helper.get_resource_tags(state_machine_arn)
        tags.assert_ack_system_tags(
            tags=state_machine_tags,
            key_member_name = 'key',
            value_member_name  = 'value'
        )
        tags.assert_equal_without_ack_tags(
            actual=cr["spec"]["tags"],
            expected=state_machine_tags,
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
        # update tracing configuration
        cr["spec"]["tracingConfiguration"] = {
            "enabled": True,
        }

        # Patch k8s resource
        k8s.patch_custom_resource(ref, cr)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)

        state_machine_tags = sfn_helper.get_resource_tags(state_machine_arn)
        tags.assert_equal_without_ack_tags(
            actual=cr["spec"]["tags"],
            expected=state_machine_tags,
            key_member_name = 'key',
            value_member_name  = 'value'
        )

        state_machine = sfn_helper.get_state_machine(cr["status"]["ackResourceMetadata"]["arn"])
        assert state_machine["tracingConfiguration"]["enabled"]

        # Update definition to verify is_document comparison works
        updates = {
            "spec": {"definition": UPDATED_DEFINITION},
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(UPDATE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(ref, "ACK.ResourceSynced", "True", wait_periods=5)

        # Delete k8s resource
        _, deleted = k8s.delete_custom_resource(ref)
        assert deleted is True

        time.sleep(DELETE_WAIT_AFTER_SECONDS)

        # Check state machine is deleting
        status = sfn_helper.get_state_machine_status(state_machine_arn)
        assert status is None or status == "DELETING"

    def test_create_with_publish_records_version(
        self, sfn_client, state_machine_published
    ):
        """publish=true at create cuts a version and records its ARN."""
        ref, cr = state_machine_published
        helper = SFNHelper(sfn_client)
        sm_arn = cr["status"]["ackResourceMetadata"]["arn"]

        versions = helper.list_state_machine_versions(sm_arn)
        assert len(versions) == 1

        recorded = _version_arn(ref)
        assert recorded == versions[0]
        assert helper.state_machine_version_exists(recorded)

        # Nothing is owed, so a further read must not publish again.
        _force_reconcile(ref)
        time.sleep(SETTLE_WAIT_AFTER_SECONDS)
        assert helper.list_state_machine_versions(sm_arn) == [recorded]
        assert _version_arn(ref) == recorded

    def test_no_publish_means_no_version(self, sfn_client, state_machine_unpublished):
        """publish=false cuts nothing and records nothing."""
        ref, cr = state_machine_unpublished
        helper = SFNHelper(sfn_client)
        sm_arn = cr["status"]["ackResourceMetadata"]["arn"]

        assert helper.list_state_machine_versions(sm_arn) == []
        assert _version_arn(ref) is None

        # publish is off, so the probe is skipped and nothing is ever cut.
        _force_reconcile(ref)
        time.sleep(SETTLE_WAIT_AFTER_SECONDS)
        assert helper.list_state_machine_versions(sm_arn) == []
        assert _version_arn(ref) is None

    def test_definition_change_publishes_new_version(
        self, sfn_client, state_machine_published
    ):
        """A spec change with publish=true cuts a version, and only one."""
        ref, cr = state_machine_published
        helper = SFNHelper(sfn_client)
        sm_arn = cr["status"]["ackResourceMetadata"]["arn"]
        first = _version_arn(ref)

        k8s.patch_custom_resource(ref, {"spec": {"definition": UPDATED_DEFINITION}})
        time.sleep(SETTLE_WAIT_AFTER_SECONDS)
        assert k8s.wait_on_condition(
            ref, condition.CONDITION_TYPE_RESOURCE_SYNCED, "True", wait_periods=5
        )

        second = _wait_for(
            lambda: (lambda v: v if v != first else None)(_version_arn(ref)),
            "the recorded version ARN to advance after a definition change",
        )
        assert sorted(helper.list_state_machine_versions(sm_arn)) == sorted(
            [first, second]
        )

        # Re-reading must not publish again: AWS is idempotent per revision and
        # an unchanged spec produces no delta.
        _force_reconcile(ref)
        time.sleep(SETTLE_WAIT_AFTER_SECONDS)
        assert sorted(helper.list_state_machine_versions(sm_arn)) == sorted(
            [first, second]
        )
        assert _version_arn(ref) == second

    def test_publish_bootstraps_existing_state_machine(
        self, sfn_client, state_machine_unpublished
    ):
        """Turning publish on for a settled state machine must publish.

        There is no spec difference to act on here: the definition is unchanged
        and publish itself is excluded from the comparison. Without the delta
        customPreCompare derives from a missing version ARN this silently does
        nothing, so this asserts the flip is not a no-op.
        """
        ref, cr = state_machine_unpublished
        helper = SFNHelper(sfn_client)
        sm_arn = cr["status"]["ackResourceMetadata"]["arn"]

        assert helper.list_state_machine_versions(sm_arn) == []

        k8s.patch_custom_resource(ref, {"spec": {"publish": True}})
        time.sleep(SETTLE_WAIT_AFTER_SECONDS)

        recorded = _wait_for(
            lambda: _version_arn(ref),
            "a version to be published after enabling spec.publish",
        )
        assert helper.list_state_machine_versions(sm_arn) == [recorded]
        assert helper.state_machine_version_exists(recorded)

        assert k8s.wait_on_condition(
            ref, condition.CONDITION_TYPE_RESOURCE_SYNCED, "True", wait_periods=5
        )

        # The delta must stop firing once the ARN is recorded again, otherwise
        # the controller publishes on every reconcile.
        _force_reconcile(ref)
        time.sleep(SETTLE_WAIT_AFTER_SECONDS)
        assert helper.list_state_machine_versions(sm_arn) == [recorded]
        assert _version_arn(ref) == recorded

    def test_out_of_band_version_delete_is_reconciled(
        self, sfn_client, state_machine_published
    ):
        """A version deleted outside ACK is republished.

        Version numbers are never reused, so recovery produces a new ARN rather
        than restoring the old one.
        """
        ref, cr = state_machine_published
        helper = SFNHelper(sfn_client)
        sm_arn = cr["status"]["ackResourceMetadata"]["arn"]

        original = _version_arn(ref)
        assert original is not None
        assert helper.state_machine_version_exists(original)

        helper.delete_state_machine_version(original)
        assert _wait_for(
            lambda: not helper.state_machine_version_exists(original),
            "the out-of-band version delete to take effect in AWS",
        )

        _force_reconcile(ref)

        replacement = _wait_for(
            lambda: (lambda v: v if v and v != original else None)(_version_arn(ref)),
            "the deleted version to be republished under a new ARN",
        )
        assert replacement != original
        assert helper.state_machine_version_exists(replacement)
        assert helper.list_state_machine_versions(sm_arn) == [replacement]

        assert k8s.wait_on_condition(
            ref, condition.CONDITION_TYPE_RESOURCE_SYNCED, "True", wait_periods=5
        )

        # Recovery must happen once, not on a loop.
        _force_reconcile(ref)
        time.sleep(SETTLE_WAIT_AFTER_SECONDS)
        assert helper.list_state_machine_versions(sm_arn) == [replacement]
        assert _version_arn(ref) == replacement
