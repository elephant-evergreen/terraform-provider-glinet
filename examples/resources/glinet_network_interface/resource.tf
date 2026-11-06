resource "glinet_network_interface" "nas" {
  name         = "vlan11"
  device       = "br-vlan11"
  protocol     = "static"
  address      = "192.168.11.1/24"
  vlan_id      = 11
  display_name = "NAS"
  enabled      = true
}
