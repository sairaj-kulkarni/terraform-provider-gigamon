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



# Example usge for image resource

# Define the usage of Gigamon Provider. For now refering to it from a local source
# i.e. the local file system
terraform {
  required_providers {
    gigamon = {
      source = "local/gigamon/gigamon"
    }
  }
}

# Provide the provider required parameters. Currently we support api_token based
# authentication to FM. The user must login to FM and generate the token and provide it
# here.

# please note that in this example the api_token and other sensitive information like
# passwords are provided in plain text. This is only for sample and production environment
# should use secure mecahnisms like vault

provider "gigamon" {
  fm_address = "10.114.202.120"

  # skip_verify is default false, which implies that the certificate presented by FM must be
  # a valid certificate and will be verified. For demo purpose this is skipped, but should not
  # be set in productino environment
  skip_verify = true

  # this token is generated using FM API, via  the user management section. For best
  # security rotate this token often and also use mecahnisms like vault to prevent exposing
  # this in plain text in the configuration files
  api_token = "eyJhbGciOiJIUzI1NiJ9.eyJ0b2tlbklkIjoiOTIxNjgzMDk0MjA0ODQ3NSIsInN1YiI6InRmLXRva2VuIiwiaWF0IjoxNzYyMzMwMjk4LCJleHAiOjE3NjQ5MjIyOTh9.WPPhWxx_MeG40RgIJYZVm0zt1v-ahyutPRQzUVWVf_0"
}

import {
  to = gigamon_esxi_monitoring_domain.my-md
  id = "3f7f128d-1712-44a3-955e-00f13bec6ad4"
}

resource "gigamon_esxi_monitoring_domain" "my-md" {
  alias = "jana-md"
  use_public_ip_for_notifications = true
}
