resource "glinet_wireguard_interface" "wg0" {
  name           = "wg0"
  addresses      = ["10.8.0.1/24"]
  listen_port    = 51820
  private_key_wo = var.wireguard_private_key
}
