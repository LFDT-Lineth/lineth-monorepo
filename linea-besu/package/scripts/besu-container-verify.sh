#!/bin/bash
##
## Copyright contributors to Hyperledger Besu.
##
## Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
## the License. You may obtain a copy of the License at
##
## http://www.apache.org/licenses/LICENSE-2.0
##
## Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
## an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
## specific language governing permissions and limitations under the License.
##
## SPDX-License-Identifier: Apache-2.0
##

CONTAINER_NAME=${CONTAINER_NAME:-besu}
RETRY=${RETRY:-10}
SLEEP=${SLEEP:-5}

# Helper function to throw error
log_error() {
  echo "::error $1"
  exit 1
}

# Check container is in running state
_RUN_STATE=$(docker inspect --type=container -f={{.State.Status}} ${CONTAINER_NAME})
if [[ "${_RUN_STATE}" != "running" ]]
then
  log_error "container is not running"
fi

# Check for specific log message in container logs to verify besu started
_SUCCESS=false
while [[ ${_SUCCESS} != "true" && $RETRY -gt 0 ]]
do
  docker logs ${CONTAINER_NAME} &> temp.out && grep -q "Ethereum main loop is up" temp.out && {
    _SUCCESS=true
    continue
  }
  echo "Waiting for the besu to start. Remaining retries $RETRY ..."
  RETRY=$(expr $RETRY - 1)
  sleep $SLEEP
done

docker logs --tail=100 ${CONTAINER_NAME}

# Log entry does not present after all retries, fail the script with a message
if [[ ${_SUCCESS} != "true" ]]
then
  log_error "could not find the log message 'Ethereum main loop is up'"
else
  echo "Besu container started and entered main loop"
fi

