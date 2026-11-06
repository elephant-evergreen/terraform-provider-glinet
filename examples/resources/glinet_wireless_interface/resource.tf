resource "glinet_wireless_interface" "private_2g" {
  section     = "wifi2g"
  ssid        = "Example"
  device      = "mt798611"
  network     = "lan"
  mode        = "ap"
  encryption  = "psk2"
  isolate     = false
  key_wo      = var.wifi_private_key
  key_version = 1
}
