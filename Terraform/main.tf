terraform {
  required_providers {
    scaleway = {
      source  = "scaleway/scaleway"
      version = "2.47.0"
    }
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "4.44.0"
    }
    local = {
      source  = "hashicorp/local"
      version = "2.5.2"
    }
    random = {
      source  = "hashicorp/random"
      version = "3.6.3"
    }
  }
}

provider "cloudflare" {
}

provider "scaleway" {
  region = "fr-par"
  zone   = "fr-par-1"
}

