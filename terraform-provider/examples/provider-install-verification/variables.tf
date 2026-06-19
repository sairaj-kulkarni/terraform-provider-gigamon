#  Copyright (c) 2017-2026 Gigamon, Inc. All rights reserved.
#
#  Author: Gigamon Terraform Team (gigamon-terraform-team@gigamon.com)
#
#  This program is free software: you can redistribute it and/or modify
#  it under the terms of the GNU General Public License as published by
#  the Free Software Foundation, version 3 of the License.
#
#  This program is distributed in the hope that it will be useful,
#  but WITHOUT ANY WARRANTY; without even the implied warranty of
#  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
#  GNU General Public License for more details.
#
#  You should have received a copy of the GNU General Public License
#  along with this program. If not, see <https://www.gnu.org/licenses/>

variable "map_rule_sets" {
  type = list(object({
    rule_set_id = string  # "1" to "5"
    priority    = number  # 1 to 5
    aep_id      = number  # 2 to 63
    pass_rules = list(object({
      rule_id = number
      ip_version = object({
        ip_version = string
      })
    }))
    drop_rules = list(object({
      rule_id = number
      ip_version = object({
        ip_version = string
      })
    }))
  }))
}
