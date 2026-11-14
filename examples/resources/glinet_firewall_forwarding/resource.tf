resource "glinet_firewall_forwarding" "guest_to_wan" {
  src     = "guest"
  dest    = "wan"
  enabled = true
}
