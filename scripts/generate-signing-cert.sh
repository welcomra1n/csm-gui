#!/usr/bin/env bash
# Generates a self-signed code signing certificate that you can reuse
# across every csm-gui release build. macOS TCC keys consent on the
# code requirement, which for a properly signed app is
# "identifier "com.wails.csm" and certificate leaf[subject.CN] = ..."
# — so signing with the SAME cert each build keeps the requirement
# stable and TCC stops re-prompting on every upgrade.
#
# Run this ONCE locally, then:
#   1. add the resulting CSM_MAC_SIGNING_CERT_P12 (base64 of the .p12)
#      and CSM_MAC_SIGNING_CERT_PWD secrets to the GitHub repo
#   2. delete the local .p12 / .pem files
#
# Adhoc-signed builds are unaffected; the workflow falls back to adhoc
# when the secrets are missing.

set -euo pipefail

NAME="CSM Self-Signed (welcomra1n)"
DAYS=3650
OUT_DIR="${1:-./signing-cert}"
PASSWORD="${CSM_CERT_PASSWORD:-$(openssl rand -base64 24)}"

mkdir -p "$OUT_DIR"
cd "$OUT_DIR"

# 1. Generate a private key + self-signed cert with codeSigning EKU.
cat > codesigning.ext <<'EOF'
[ codesigning ]
keyUsage = critical, digitalSignature
extendedKeyUsage = critical, codeSigning
basicConstraints = critical, CA:FALSE
EOF

openssl req -new -newkey rsa:2048 -nodes \
  -keyout key.pem \
  -x509 -days "$DAYS" \
  -subj "/CN=$NAME" \
  -extensions codesigning \
  -config <(cat /etc/ssl/openssl.cnf; echo; cat codesigning.ext) \
  -out cert.pem

# 2. Bundle into a .p12 with a password.
openssl pkcs12 -export \
  -inkey key.pem \
  -in cert.pem \
  -name "$NAME" \
  -passout "pass:$PASSWORD" \
  -out cert.p12

# 3. Emit base64 + password for the GH secrets.
base64 -i cert.p12 > cert.p12.b64
echo "$PASSWORD" > cert.password

cat <<EOM

──────────────────────────────────────────────────────────────────────
Cert generated in $OUT_DIR.
Next:
  1. gh secret set CSM_MAC_SIGNING_CERT_P12 < cert.p12.b64
  2. gh secret set CSM_MAC_SIGNING_CERT_PWD < cert.password
  3. SHRED these files (they're sensitive):
     shred -uvz $OUT_DIR/key.pem $OUT_DIR/cert.p12 $OUT_DIR/cert.p12.b64 $OUT_DIR/cert.password
     (or rm if shred unavailable)
Subsequent CI runs will codesign with this cert; install it once in
your local keychain too so dev builds match the CI cert hash.
──────────────────────────────────────────────────────────────────────
EOM
