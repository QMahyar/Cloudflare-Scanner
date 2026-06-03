#!/usr/bin/env sh
# Cloudflare Scanner — one-liner installer for macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-macos.sh | sh

set -e

REPO="QMahyar/Cloudflare-Scanner"
BIN_NAME="cloudflare-scanner"
INSTALL_DIR="$HOME/.local/share/cloudflare-scanner"
BIN_DIR="/usr/local/bin"

# ── detect architecture ────────────────────────────────────────────────────────
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)         PLATFORM="darwin-amd64" ;;
  arm64)          PLATFORM="darwin-arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1 ;;
esac

# ── get latest release tag ─────────────────────────────────────────────────────
echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$TAG" ]; then
  echo "Could not determine latest version. Check your internet connection."
  exit 1
fi

echo "Installing Cloudflare Scanner ${TAG} (${PLATFORM})..."

# ── download & extract ─────────────────────────────────────────────────────────
URL="https://github.com/${REPO}/releases/download/${TAG}/Cloudflare-Scanner-${TAG}-${PLATFORM}.tar.gz"

mkdir -p "$INSTALL_DIR"
curl -fsSL "$URL" | tar -xz -C "$INSTALL_DIR"
chmod +x "$INSTALL_DIR/Cloudflare-Scanner" "$INSTALL_DIR/xray"

# ── remove macOS quarantine (Gatekeeper) ──────────────────────────────────────
xattr -dr com.apple.quarantine "$INSTALL_DIR/Cloudflare-Scanner" 2>/dev/null || true
xattr -dr com.apple.quarantine "$INSTALL_DIR/xray" 2>/dev/null || true

# ── install wrapper ───────────────────────────────────────────────────────────
# Try /usr/local/bin first; fall back to ~/bin if no write access
if [ -w "$BIN_DIR" ] || sudo test -w "$BIN_DIR" 2>/dev/null; then
  WRAPPER_DEST="$BIN_DIR/$BIN_NAME"
  echo "Installing command to $BIN_DIR (may ask for sudo)..."
  sudo tee "$WRAPPER_DEST" > /dev/null << WRAPPER
#!/usr/bin/env sh
exec "$INSTALL_DIR/Cloudflare-Scanner" "\$@"
WRAPPER
  sudo chmod +x "$WRAPPER_DEST"
else
  BIN_DIR="$HOME/bin"
  mkdir -p "$BIN_DIR"
  WRAPPER_DEST="$BIN_DIR/$BIN_NAME"
  cat > "$WRAPPER_DEST" << WRAPPER
#!/usr/bin/env sh
exec "$INSTALL_DIR/Cloudflare-Scanner" "\$@"
WRAPPER
  chmod +x "$WRAPPER_DEST"
  echo "  Add ~/bin to your PATH: export PATH=\"\$HOME/bin:\$PATH\""
fi

echo ""
echo "Done! Run:  $BIN_NAME"
echo "Update:    re-run this installer"
echo "Uninstall: rm -rf $INSTALL_DIR $WRAPPER_DEST"
