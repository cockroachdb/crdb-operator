#!/usr/bin/env bash

# Copyright 2020 Coachroach Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# "---------------------------------------------------------"
# "-                                                       -"
# "-  deploy the operator 				   -"
# "-                                                       -"
# "---------------------------------------------------------"

set -o errexit
set -o nounset
set -o pipefail

ROOT=$(dirname "${BASH_SOURCE[0]}")
CLUSTER_NAME=""
# TODO maybe set default zone?
ZONE=""
IMAGE_NAME=""

# shellcheck disable=SC1090
source "$ROOT"/common.sh

# If user did not pass in -i flag then fail
if [ -z "${IMAGE_NAME}" ]; then
  echo "Usage: $0 [-c <cluster name> -i <image_name>]" 1>&2; exit 1;
fi

gcloud container clusters get-credentials "$CLUSTER_NAME" --zone "$ZONE"

kubectl apply -f ${ROOT}/../config/crd/bases/crdb.cockroachlabs.com_crdbclusters.yaml
# TODO I do not like the cd here, but I do not know how to do an edit without being
# in the directory.
cd ${ROOT}/../deploy
kustomize edit set image cockroach-operator=${IMAGE_NAME}
kustomize build ${ROOT}/../deploy | kubectl apply -f -

# TODO test validating operator
ATTEMPTS=0
ROLLOUT_STATUS_CMD="kubectl rollout status deployment/cockroach-operator -n default"
until $ROLLOUT_STATUS_CMD || [ $ATTEMPTS -eq 60 ]; do
  $ROLLOUT_STATUS_CMD
  ATTEMPTS=$((attempts + 1))
  sleep 10
done

