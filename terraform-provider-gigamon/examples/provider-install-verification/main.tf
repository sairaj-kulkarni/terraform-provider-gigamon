terraform {
  required_providers {
    gigamon = {
	  source = "local/gigamon/gigamon"
	}
  }
}

provider "gigamon" {
# Does not accept any configuration as of now
}

data "gigamon_example" "example" {
}

resource "gigamon_example" "example1" {
}
