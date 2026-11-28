terraform {
  required_providers {
    glinet = {
      source  = "elephant-evergreen/glinet"
      version = "~> 0.2"
    }
  }
}

provider "glinet" {}

data "glinet_system" "router" {}

output "router" {
  value = {
    model    = data.glinet_system.router.model
    firmware = data.glinet_system.router.firmware
    mode     = data.glinet_system.router.mode
  }
}
