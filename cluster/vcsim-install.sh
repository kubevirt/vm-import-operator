#!/bin/bash
#
# Copyright 2018-2020 Red Hat, Inc.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

KUBECTL=${KUBECTL:-'./cluster/kubectl.sh'}

if [ ! -d "_out" ]
then
  mkdir _out
fi

sed 's/@VCSIM_NAMESPACE/'"$1"'/' cluster/manifests/vcsim.yaml.in > _out/vcsim.yaml
$KUBECTL apply -f _out/vcsim.yaml
$KUBECTL -n "$1" wait deploy vcsim-deployment --for condition=Available --timeout=1200s || debug "$1"
$KUBECTL -n "$1" port-forward service/vcsim 8989:8989 &