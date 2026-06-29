output "name" {
  description = "VM instance name"
  value       = incus_instance.vm.name
}

output "ipv4_address" {
  description = "VM IPv4 address on the bridge network"
  value       = incus_instance.vm.ipv4_address
}

output "status" {
  description = "VM running status"
  value       = incus_instance.vm.status
}
