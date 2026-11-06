terraform {
  required_providers {
    glinet = {
      source  = "elephant-evergreen/glinet"
      version = "= 0.1.0"
    }
  }
}

provider "glinet" {}
