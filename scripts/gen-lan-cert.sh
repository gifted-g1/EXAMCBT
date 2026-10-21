#!/usr/bin/env bash
# Creates a private certificate authority and a server certificate for the
# exam server's LAN IP, so browsers allow webcam access (secure context).
#
#   ./scripts/gen-lan-cert.sh 192.168.1.45 [more IPs...]
#
# Output (deploy/certs/):
#   ca.crt      install this as a trusted root on exam-hall PCs (GPO / MDM),
#               once per institution; keep ca.key offline.
#   server.crt  + server.key  for Nginx (compose) or TLS_CERT_FILE/TLS_KEY_FILE.
#
# Without installing ca.crt, students see a certificate warning; with it,
# http://<ip> → https://<ip> just works.
set -euo pipefail
[ $# -ge 1 ] || { echo "usage: $0 <lan-ip> [ip...]"; exit 2; }
out="$(cd "$(dirname "$0")/.." && pwd)/deploy/certs"
mkdir -p "$out"
cd "$out"

if [ ! -f ca.key ]; then
  openssl req -x509 -newkey rsa:3072 -sha256 -days 3650 -nodes \
    -keyout ca.key -out ca.crt -subj "/CN=Exam Shield Local CA"
  chmod 600 ca.key
fi

san=""
for ip in "$@"; do san="${san:+$san,}IP:$ip"; done

openssl req -newkey rsa:2048 -sha256 -nodes -keyout server.key -out server.csr \
  -subj "/CN=$1"
openssl x509 -req -in server.csr -CA ca.crt -CAkey ca.key -CAcreateserial \
  -days 397 -sha256 -out server.crt \
  -extfile <(printf "subjectAltName=%s\nextendedKeyUsage=serverAuth\nbasicConstraints=CA:FALSE\n" "$san")
rm -f server.csr
chmod 600 server.key
echo "Wrote $out/server.crt (SAN: $san). Distribute $out/ca.crt to exam PCs."
