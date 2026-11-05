resource "glinet_uci_configuration" "example" {
  configuration = jsonencode([
    {
      package = "wireless"
      name    = "guest2g"
      type    = "wifi-iface"
      options = { ssid = "Example Guest", disabled = "0", hidden = "0", isolate = "1" }
    },
    {
      package = "firewall"
      name    = "example_guest_management"
      type    = "rule"
      options = { name = "Example guest management", src = "guest", proto = "tcp", dest_port = "22 80 443", target = "REJECT" }
    }
  ])
}
