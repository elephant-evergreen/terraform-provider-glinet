resource "glinet_wireless_interface_settings" "guest_2g" {
  section  = "guest2g"
  ssid     = "example-guest"
  disabled = false
  hidden   = false
  isolate  = true
}
