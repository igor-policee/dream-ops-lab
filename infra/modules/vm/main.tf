resource "incus_instance" "vm" {
  name    = var.name
  image   = var.image
  type    = "virtual-machine"
  running = true

  profiles = var.profiles

  config = merge(
    {
      "limits.cpu"    = tostring(var.cpu)
      "limits.memory" = var.memory
    },
    var.user_data != "" ? {
      "cloud-init.user-data" = var.user_data
    } : {}
  )

  device {
    name = "root"
    type = "disk"

    properties = {
      pool = var.storage_pool
      path = "/"
      size = var.disk_size
    }
  }

  device {
    name = "eth0"
    type = "nic"

    properties = {
      network        = var.network
      name           = "eth0"
      "ipv4.address" = var.ipv4_address
    }
  }
}
