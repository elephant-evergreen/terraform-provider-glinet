resource "glinet_wireless_radio" "wifi_2g" {
  device  = "mt798611"
  channel = "auto"
  htmode  = "HE40"
  txpower = 100
  country = "JP"
}
