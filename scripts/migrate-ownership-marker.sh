#!/bin/sh
set -eu
if [ "$(id -u)" -ne 0 ]; then
  echo "Run this migration as root on the router." >&2
  exit 1
fi
packages="network wireless dhcp firewall gl-dns-v2 eth_ports_config_map"
for package in $packages; do
  uci show "$package" 2>/dev/null | grep -F ".home_iac_owner='1'" | while IFS= read -r line; do
    section=${line%%=*}
    section=${section%.home_iac_owner}
    section=${section#"$package".}
    uci set "$package.$section.terraform_owner=1"
    uci delete "$package.$section.home_iac_owner"
  done
  uci commit "$package"
done
remaining=0
for package in $packages; do
  count=$(uci show "$package" 2>/dev/null | grep -cF ".home_iac_owner=" || true)
  remaining=$((remaining + count))
done
if [ "$remaining" -ne 0 ]; then
  echo "Legacy ownership markers remain: $remaining" >&2
  exit 1
fi
echo "Ownership markers migrated to terraform_owner."
