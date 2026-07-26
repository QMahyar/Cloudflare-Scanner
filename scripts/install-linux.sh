#!/usr/bin/env sh
# Cloudflare Scanner — one-liner installer for Linux
# Usage: curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-linux.sh | sh

set -e

REPO="QMahyar/Cloudflare-Scanner"
INSTALL_DIR="$HOME/.local/share/cloudflare-scanner"
BIN_DIR="$HOME/.local/bin"

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

case "$(uname -m)" in
x86_64) PLATFORM="linux-amd64" ;;
aarch64 | arm64) PLATFORM="linux-arm64" ;;
*) echo "Unsupported Linux architecture: $(uname -m)"; exit 1 ;;
esac

echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
[ -n "$TAG" ] || { echo "Could not determine latest version."; exit 1; }

echo "Installing Cloudflare Scanner ${TAG} (${PLATFORM})..."
URL="https://github.com/${REPO}/releases/download/${TAG}/Cloudflare-Scanner-${TAG}-${PLATFORM}.tar.gz"
mkdir -p "$INSTALL_DIR" "$BIN_DIR"
curl -fsSL "$URL" | tar -xz -C "$INSTALL_DIR"
chmod +x "$INSTALL_DIR/Cloudflare-Scanner" "$INSTALL_DIR/xray"
curl -fsSL "https://raw.githubusercontent.com/${REPO}/master/scripts/scan-command.sh" -o "$INSTALL_DIR/scan-command"
chmod +x "$INSTALL_DIR/scan-command"

for name in Cloudflare-Scanner cloudflare-scanner scan; do
	cat >"$BIN_DIR/$name" <<WRAPPER
#!/usr/bin/env sh
exec "$INSTALL_DIR/scan-command" "\$@"
WRAPPER
	chmod +x "$BIN_DIR/$name"
done

case ":$PATH:" in
*":$BIN_DIR:"*) ;;
*)
	echo ""
	echo "Add ~/.local/bin to your PATH by appending this to ~/.bashrc or ~/.zshrc:"
	echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
	echo ""
	;;
esac

echo "Done! Run: Cloudflare-Scanner  (aliases: cloudflare-scanner, scan)"
echo "Commands: Cloudflare-Scanner help | update | restart | uninstall"
