# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Helper functions for SFN e2e tests
"""

import logging

class SFNHelper:
    def __init__(self, sfn_client):
        self.sfn_client = sfn_client

    def get_activity(self, activity_arn: str) -> dict:
        try:
            resp = self.sfn_client.describe_activity(
                activityArn=activity_arn
            )
            return resp

        except Exception as e:
            logging.debug(e)
            return None

    def get_resource_tags(self, activity_arn: str):
        resource_tags = self.sfn_client.list_tags_for_resource(
            resourceArn=activity_arn,
        )
        return resource_tags['tags']

    def activity_exists(self, activity_arn) -> bool:
        return self.get_activity(activity_arn) is not None

    def get_state_machine(self, state_machine_arn: str) -> dict:
        try:
            resp = self.sfn_client.describe_state_machine(
                stateMachineArn=state_machine_arn
            )
            return resp

        except Exception as e:
            logging.debug(e)
            return None

    def state_machine_exists(self, state_machine_arn) -> bool:
        return self.get_state_machine(state_machine_arn) is not None

    def publish_state_machine_version(self, state_machine_arn: str, description: str = "") -> dict:
        try:
            resp = self.sfn_client.publish_state_machine_version(
                stateMachineArn=state_machine_arn,
                description=description,
            )
            return resp
        except Exception as e:
            logging.debug(e)
            return None

    def delete_state_machine_version(self, version_arn: str):
        try:
            self.sfn_client.delete_state_machine_version(
                stateMachineVersionArn=version_arn
            )
        except Exception as e:
            logging.debug(e)

    def list_state_machine_versions(self, state_machine_arn: str) -> list:
        """Return every version ARN for a state machine, newest first.

        Paged by hand because botocore ships no paginator for
        ListStateMachineVersions. The API sorts by descending version creation
        time and returns nextToken while pages remain.

        Errors are not swallowed: callers assert on an empty list to mean "no
        versions exist", so a failed call must not be indistinguishable from
        that.
        """
        arns = []
        next_token = None
        while True:
            kwargs = {"stateMachineArn": state_machine_arn}
            if next_token is not None:
                kwargs["nextToken"] = next_token
            resp = self.sfn_client.list_state_machine_versions(**kwargs)
            arns.extend(
                v["stateMachineVersionArn"] for v in resp["stateMachineVersions"]
            )
            next_token = resp.get("nextToken")
            if not next_token:
                return arns

    def state_machine_version_exists(self, version_arn: str) -> bool:
        """A version ARN describes like a state machine; a missing one raises
        StateMachineDoesNotExist."""
        return self.get_state_machine(version_arn) is not None

    def describe_state_machine_alias(self, alias_arn: str) -> dict:
        try:
            resp = self.sfn_client.describe_state_machine_alias(
                stateMachineAliasArn=alias_arn
            )
            return resp
        except Exception as e:
            logging.debug(e)
            return None

    def state_machine_alias_exists(self, alias_arn: str) -> bool:
        return self.describe_state_machine_alias(alias_arn) is not None

    def get_state_machine_status(self, state_machine_arn: str) -> str:
        """Return the status of a state machine (e.g. 'ACTIVE', 'DELETING'), or None if not found."""
        sm = self.get_state_machine(state_machine_arn)
        if sm is None:
            return None
        return sm.get("status")