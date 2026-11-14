resource "glinet_network_bridge" "lan" {
  name  = "br-lan"
  ports = ["lan1", "lan2"]
}
