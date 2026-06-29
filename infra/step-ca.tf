module "step_ca" {
  source = "./modules/vm"

  name         = "step-ca-01"
  image        = var.vm_image
  cpu          = 1
  memory       = "1GB"
  disk_size    = "10GB"
  storage_pool = var.storage_pool
  network      = var.network
  ipv4_address = "10.10.0.10"
}

output "step_ca_ip" {
  value = module.step_ca.ipv4_address
}
