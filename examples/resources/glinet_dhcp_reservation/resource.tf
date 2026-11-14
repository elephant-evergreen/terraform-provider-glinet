resource "glinet_dhcp_reservation" "printer" {
  section  = "printer"
  hostname = "printer"
  mac      = "02:00:00:00:00:20"
  ip       = "192.168.11.20"
}
