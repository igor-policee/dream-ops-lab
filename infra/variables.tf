variable "storage_pool" {
  description = "Incus storage pool for all VMs"
  type        = string
  default     = "incus-pool"
}

variable "network" {
  description = "Incus network for all VMs"
  type        = string
  default     = "incusbr0"
}

variable "vm_image" {
  description = "Default OS image for pre-K8s VMs"
  type        = string
  default     = "images:ubuntu/24.04/cloud"
}
