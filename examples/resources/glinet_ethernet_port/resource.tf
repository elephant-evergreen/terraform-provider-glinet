resource "glinet_ethernet_port" "lan2" {
  port      = "lan2"
  mode      = "lan"
  vlan_mode = "access"
  pvid      = 1
}
