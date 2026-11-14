resource "glinet_firewall_zone" "guest" {
  name     = "guest"
  networks = ["guest"]
  input    = "REJECT"
  output   = "ACCEPT"
  forward  = "REJECT"
}
