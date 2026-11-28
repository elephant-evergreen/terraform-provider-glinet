resource "glinet_samba_share" "media" {
  section         = "media_share"
  share_name      = "media"
  path            = "/mnt/media"
  read_only       = "yes"
  guest_ok        = "no"
  delete_behavior = "delete"
}
