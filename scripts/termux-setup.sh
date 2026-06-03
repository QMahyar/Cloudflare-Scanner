#!/data/data/com.termux/files/usr/bin/sh
# Cloudflare Scanner — one-liner installer for Termux (Android)
# Usage: curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh

set -e

REPO="QMahyar/Cloudflare-Scanner"
INSTALL_DIR="$HOME/.local/share/cloudflare-scanner"
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"

pkg update -y -o Dpkg::Use-Pty=0 2>/dev/null || pkg update -y
pkg install -y curl 2>/dev/null || true

echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$TAG" ]; then
  echo "Could not determine latest version."
  exit 1
fi

echo "Installing Cloudflare Scanner ${TAG} (Termux / Android arm64)..."

URL="https://github.com/${REPO}/releases/download/${TAG}/Cloudflare-Scanner-${TAG}-termux-arm64.tar.gz"

mkdir -p "$INSTALL_DIR"
curl -fsSL "$URL" | tar -xz -C "$INSTALL_DIR"
chmod +x "$INSTALL_DIR/Cloudflare-Scanner" "$INSTALL_DIR/xray"

# Create 'scan' command in Termux's bin directory
cat > "$PREFIX/bin/scan" << 'WRAPPER'
#!/data/data/com.termux/files/usr/bin/sh
exec "$HOME/.local/share/cloudflare-scanner/Cloudflare-Scanner" "$@"
WRAPPER
chmod +x "$PREFIX/bin/scan"

echo ""
echo "Done! Type: scan"
echo ""
echo "Update:    re-run this installer"
echo "Uninstall: rm -rf $INSTALL_DIR && rm $PREFIX/bin/scan"
