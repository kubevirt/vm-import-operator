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

sed 's/@IMAGEIO_NAMESPACE/'"$1"'/' cluster/manifests/imageio.yaml.in > _out/imageio.yaml
$KUBECTL apply -f _out/imageio.yaml
$KUBECTL -n "$1" wait deploy imageio-deployment --for condition=Available --timeout=1200s || debug "$1"
$KUBECTL -n "$1" port-forward service/imageio 12346:12346 &