resource "glinet_wireless_interface" "private_2g" {
  section        = "wifi2g"
  ssid           = "Example"
  device         = "mt798611"
  network        = "lan"
  mode           = "ap"
  encryption     = "sae"
  ieee80211w     = 2
  wps_pushbutton = false
  isolate        = false
}
