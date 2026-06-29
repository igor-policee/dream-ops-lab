variable "name" {
  description = "VM hostname"
  type        = string
}

variable "image" {
  description = "Incus image reference (e.g. images:ubuntu/24.04/cloud)"
  type        = string
  default     = "images:ubuntu/24.04/cloud"
}

variable "cpu" {
  description = "Number of vCPUs"
  type        = number
}

variable "memory" {
  description = "RAM size (e.g. 2GB)"
  type        = string
}

variable "disk_size" {
  description = "Root disk size (e.g. 20GB)"
  type        = string
}

variable "storage_pool" {
  description = "Incus storage pool name"
  type        = string
  default     = "incus-pool"
}

variable "network" {
  description = "Incus network name for eth0"
  type        = string
  default     = "incusbr0"
}

variable "ipv4_address" {
  description = "Static IPv4 address on the bridge network"
  type        = string
}

variable "user_data" {
  description = "cloud-init user-data (YAML string)"
  type        = string
  default     = ""
}

variable "profiles" {
  description = "Incus profiles to apply"
  type        = list(string)
  default     = ["default"]
}
