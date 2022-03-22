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



function build::common::get_go_path() {
  local -r version=$1
  local gobinaryversion=""

  if [[ $version == "1.13"* ]]; then
    gobinaryversion="1.13"
  fi
  if [[ $version == "1.14"* ]]; then
    gobinaryversion="1.14"
  fi
  if [[ $version == "1.15"* ]]; then
    gobinaryversion="1.15"
  fi
  if [[ $version == "1.16"* ]]; then
    gobinaryversion="1.16"
  fi
  if [[ $version == "1.17"* ]]; then
    gobinaryversion="1.17"
  fi

  if [[ "$gobinaryversion" == "" ]]; then
    return
  fi

  # This is the path where the specific go binary versions reside in our builder-base image
  local -r gorootbinarypath="/go/go${gobinaryversion}/bin"
  # This is the path that will most likely be correct if running locally
  local -r gopathbinarypath="$GOPATH/go${gobinaryversion}/bin"
  if [ -d "$gorootbinarypath" ]; then
    echo $gorootbinarypath
  elif [ -d "$gopathbinarypath" ]; then
    echo $gopathbinarypath
  else
    # not in builder-base, probably running in dev environment
    # return default go installation
    local -r which_go=$(which go)
    echo "$(dirname $which_go)"
  fi
}