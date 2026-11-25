resource "glinet_network_rule" "work" {
  name     = "work_table"
  lookup   = "100"
  priority = 1000
  src      = "10.0.0.0/8"
  in       = glinet_network_interface.ni_lan.name
  family   = "ipv4"
}
