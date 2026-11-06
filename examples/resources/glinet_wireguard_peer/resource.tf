resource "glinet_wireguard_peer" "peer" {
  section          = "peer1"
  interface        = "wg0"
  public_key       = var.wireguard_peer_public_key
  allowed_ips      = ["10.8.0.2/32"]
  endpoint_host    = "vpn.example"
  endpoint_port    = 51820
  preshared_key_wo = var.wireguard_preshared_key
}
