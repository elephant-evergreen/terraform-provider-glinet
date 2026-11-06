data "glinet_system" "router" {}

output "firmware" {
  value = data.glinet_system.router.firmware
}
