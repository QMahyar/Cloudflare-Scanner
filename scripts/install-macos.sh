#!/usr/bin/env sh
# Cloudflare Scanner — one-liner installer for macOS
# Usage: curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/install-macos.sh | sh

set -e

REPO="QMahyar/Cloudflare-Scanner"
INSTALL_DIR="$HOME/.local/share/cloudflare-scanner"
BIN_DIR="/usr/local/bin"

if [ "$(uname -s)" != "Darwin" ]; then
	echo "This installer is for macOS only."
	echo "Linux:   run scripts/install-linux.sh."
	echo "Windows: run scripts/install-windows.ps1 in PowerShell."
	exit 1
fi
case "$(uname -m)" in
x86_64) PLATFORM="darwin-amd64" ;;
arm64) PLATFORM="darwin-arm64" ;;
*) echo "Unsupported macOS architecture: $(uname -m)"; exit 1 ;;
esac

echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
[ -n "$TAG" ] || { echo "Could not determine latest version."; exit 1; }

echo "Installing Cloudflare Scanner ${TAG} (${PLATFORM})..."
URL="https://github.com/${REPO}/releases/download/${TAG}/Cloudflare-Scanner-${TAG}-${PLATFORM}.tar.gz"
mkdir -p "$INSTALL_DIR"
curl -fsSL "$URL" | tar -xz -C "$INSTALL_DIR"
chmod +x "$INSTALL_DIR/Cloudflare-Scanner" "$INSTALL_DIR/xray"
xattr -dr com.apple.quarantine "$INSTALL_DIR/Cloudflare-Scanner" "$INSTALL_DIR/xray" 2>/dev/null || true
curl -fsSL "https://raw.githubusercontent.com/${REPO}/master/scripts/scan-command.sh" -o "$INSTALL_DIR/scan-command"
chmod +x "$INSTALL_DIR/scan-command"

install_wrappers() {
	dir=$1
	for name in Cloudflare-Scanner cloudflare-scanner scan; do
		cat >"$dir/$name" <<WRAPPER
#!/usr/bin/env sh
exec "$INSTALL_DIR/scan-command" "\$@"
WRAPPER
		chmod +x "$dir/$name"
	done
}

if [ -w "$BIN_DIR" ] || sudo test -w "$BIN_DIR" 2>/dev/null; then
	echo "Installing commands to $BIN_DIR (may ask for sudo)..."
	sudo mkdir -p "$BIN_DIR"
	for name in Cloudflare-Scanner cloudflare-scanner scan; do
		sudo tee "$BIN_DIR/$name" >/dev/null <<WRAPPER
#!/usr/bin/env sh
exec "$INSTALL_DIR/scan-command" "\$@"
WRAPPER
		sudo chmod +x "$BIN_DIR/$name"
	done
	WRAPPER_DIR="$BIN_DIR"
else
	WRAPPER_DIR="$HOME/bin"
	mkdir -p "$WRAPPER_DIR"
	install_wrappers "$WRAPPER_DIR"
	echo "Add ~/bin to your PATH: export PATH=\"\$HOME/bin:\$PATH\""
fi

echo "Done! Run: Cloudflare-Scanner  (aliases: cloudflare-scanner, scan)"
echo "Commands: Cloudflare-Scanner help | update | restart | uninstall"
