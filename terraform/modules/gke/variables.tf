# Copyright 2022, Google LLC.
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

variable "project" {
  type        = string
  description = "Google Cloud project identifier"
}

variable "environment" {
  type        = string
  description = "system deployment identifier"
}

variable "settings" {
  type = object({
    region             = string
    location           = string
    initial_node_count = number
    min_node_count     = number
    max_node_count     = number
    machine_type       = string
  })
  description = "Google Kubernetes cluster settings"
}
