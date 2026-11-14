resource "glinet_dnsmasq" "router" {
  no_resolv = true
  servers   = ["127.0.0.1#5453"]
}
