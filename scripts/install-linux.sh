#!/usr/bin/env sh
# Cloudflare Scanner — one-liner installer for Linux
# Usage: curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-linux.sh | sh

set -e

REPO="QMahyar/Cloudflare-Scanner"
BIN_NAME="cloudflare-scanner"
INSTALL_DIR="$HOME/.local/share/cloudflare-scanner"
BIN_DIR="$HOME/.local/bin"

# ── detect platform ────────────────────────────────────────────────────────────
OS=$(uname -s)
if [ "$OS" != "Linux" ]; then
	echo "This installer is for Linux only (detected: $OS)."
	echo "Windows: run scripts/install-windows.ps1 in PowerShell."
	echo "macOS:   run scripts/install-macos.sh."
	exit 1
fi
if [ -n "${TERMUX_VERSION:-}" ] || [ -d /data/data/com.termux ]; then
	echo "This installer is for desktop/server Linux."
	echo "Termux: curl -fsSL https://raw.githubusercontent.com/${REPO}/master/scripts/termux-setup.sh | sh"
	exit 1
fi

ARCH=$(uname -m)
case "$ARCH" in
x86_64) PLATFORM="linux-amd64" ;;
aarch64 | arm64) PLATFORM="linux-arm64" ;;
*)
	echo "Unsupported Linux architecture: $ARCH"
	echo "Supported: x86_64, aarch64/arm64"
	exit 1
	;;
esac

# ── get latest release tag ─────────────────────────────────────────────────────
echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" |
	grep '"tag_name"' | cut -d'"' -f4)

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

# ── create wrapper in ~/.local/bin ─────────────────────────────────────────────
mkdir -p "$BIN_DIR"
cat >"$BIN_DIR/$BIN_NAME" <<WRAPPER
#!/usr/bin/env sh
exec "$INSTALL_DIR/Cloudflare-Scanner" "\$@"
WRAPPER
chmod +x "$BIN_DIR/$BIN_NAME"

# ── PATH hint ──────────────────────────────────────────────────────────────────
case ":$PATH:" in
*":$BIN_DIR:"*) ;;
*)
	echo ""
	echo "  Add ~/.local/bin to your PATH by appending this to ~/.bashrc or ~/.zshrc:"
	echo "    export PATH=\"\$HOME/.local/bin:\$PATH\""
	echo ""
	;;
esac

echo "Done! Run:  $BIN_NAME"
echo "Update:    re-run this installer"
echo "Uninstall: rm -rf $INSTALL_DIR $BIN_DIR/$BIN_NAME"
