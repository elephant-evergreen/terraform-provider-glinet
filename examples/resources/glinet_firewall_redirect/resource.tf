resource "glinet_firewall_redirect" "example" {
  name      = "Forward-Example-HTTPS"
  src       = "wan"
  src_dport = "8443"
  dest      = "lan"
  dest_ip   = "192.168.11.20"
  dest_port = "443"
  enabled   = false
}
