resource "glinet_network_route" "nas" {
  name      = "nas_route"
  interface = glinet_network_interface.ni_lan.name
  target    = "10.20.0.0/16"
  gateway   = "192.168.1.1"
}
