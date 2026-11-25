resource "glinet_firewall_rules" "main" {
  rules = [
    {
      name     = "Allow-DHCP-Renew"
      existing = true
    },
    {
      name       = "example-block-management"
      src        = "wan"
      protocols  = ["tcp"]
      dest_ports = ["22", "443"]
      target     = "DROP"
    },
  ]
}
