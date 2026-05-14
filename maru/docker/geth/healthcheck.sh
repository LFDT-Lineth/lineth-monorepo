peers=$(geth attach --datadir /data --exec admin.peers.length)
if [[ "$peers" -gt 0 ]]; then
  exit 0
fi
echo "No peers!"
# To restart container
kill 1
