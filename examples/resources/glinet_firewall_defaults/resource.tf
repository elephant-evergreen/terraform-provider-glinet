resource "glinet_firewall_defaults" "main" {
  input           = "REJECT"
  output          = "ACCEPT"
  forward         = "REJECT"
  drop_invalid    = true
  flow_offloading = true
}
