#!/bin/bash

ts=$(date "+%F %T")
used=$(cat /proc/sys/net/netfilter/nf_conntrack_count)
max=$(cat /proc/sys/net/netfilter/nf_conntrack_max)
percent=$(awk "BEGIN {printf \"%.2f\", $used/$max*100}")

echo "[$ts] Conntrack Usage: $used / $max ($percent%)"
echo "Top 5 src IPs:"
sudo conntrack -L | awk '{for(i=1;i<=NF;i++) if($i ~ /^src=/) {split($i,a,"="); print a[2]}}' | sort | uniq -c | sort -nr | head -5
echo
echo "Sample connections:"
sudo conntrack -L | head -5
