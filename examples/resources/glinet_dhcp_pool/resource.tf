resource "glinet_dhcp_pool" "lan" {
  section    = "lan"
  interface  = "lan"
  start      = 100
  limit      = 100
  lease_time = "12h"
}
