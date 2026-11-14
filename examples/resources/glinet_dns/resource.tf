resource "glinet_dns" "router" {
  dns_provider = "quad9"
  protocol     = "doh"
  servers      = ["quad9-doh-ip4-port443-filter-pri"]
}
