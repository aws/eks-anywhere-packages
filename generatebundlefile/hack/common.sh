#!/usr/bin/env bash

# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Common functions used by various build scripts. To use these
# functions, source this file, e.g.:
#    . <path-to-this-file>/common.sh

# awsAuth calls a command and pipes credentials to it.
#
# It will pipe its output into any command, though it was built to
# work with oras sub-commands that support the --password-stdin flag
# (e.g. pull and push). If the function detects that a public ECR is
# in use, it will adjust accordingly.
#
# An AWS profile can be selected via the PROFILE env var (defaults to
# "default").
function awsAuth () {
    declare -a cmd=( ecr )

    for arg in "$@"; do
	if [[ "$arg" =~ public\.ecr\.aws ]]; then
	    cmd=( ecr-public --region us-east-1 )
	    break
	fi
    done
    # shellcheck disable=SC2086
    aws ${cmd[*]} get-login-password --profile ${PROFILE:-default} \
	| "$@"
}
