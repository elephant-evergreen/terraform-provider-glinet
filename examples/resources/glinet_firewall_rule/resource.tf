resource "glinet_firewall_rule" "guest_dns" {
  name       = "Allow-Guest-DNS"
  src        = "guest"
  protocols  = ["tcp", "udp"]
  dest_ports = ["53"]
  target     = "ACCEPT"
}
