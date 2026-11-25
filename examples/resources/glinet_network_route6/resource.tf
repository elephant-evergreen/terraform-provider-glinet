resource "glinet_network_route6" "nas" {
  name      = "nas_route6"
  interface = glinet_network_interface.ni_lan.name
  target    = "2001:db8:20::/48"
  gateway   = "2001:db8:1::1"
}
