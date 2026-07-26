#!/data/data/com.termux/files/usr/bin/sh
# Cloudflare Scanner — one-liner installer for Termux (Android)
# Usage: curl -fsSL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh

set -e

REPO="QMahyar/Cloudflare-Scanner"
INSTALL_DIR="$HOME/.local/share/cloudflare-scanner"
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"

if ! command -v pkg >/dev/null 2>&1 || [ ! -d /data/data/com.termux ]; then
	echo "This installer is for Termux on Android only."
	exit 1
fi
case "$(uname -m)" in
aarch64 | arm64) ;;
*) echo "Unsupported Termux architecture: $(uname -m). Only ARM64 is released."; exit 1 ;;
esac

pkg update -y -o Dpkg::Use-Pty=0 2>/dev/null || pkg update -y
pkg install -y curl 2>/dev/null || true
echo "Fetching latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
[ -n "$TAG" ] || { echo "Could not determine latest version."; exit 1; }

echo "Installing Cloudflare Scanner ${TAG} (Termux / Android arm64)..."
URL="https://github.com/${REPO}/releases/download/${TAG}/Cloudflare-Scanner-${TAG}-termux-arm64.tar.gz"
mkdir -p "$INSTALL_DIR"
curl -fsSL "$URL" | tar -xz -C "$INSTALL_DIR"
chmod +x "$INSTALL_DIR/Cloudflare-Scanner" "$INSTALL_DIR/xray"
curl -fsSL "https://raw.githubusercontent.com/${REPO}/master/scripts/scan-command.sh" -o "$INSTALL_DIR/scan-command"
chmod +x "$INSTALL_DIR/scan-command"

for name in Cloudflare-Scanner cloudflare-scanner scan; do
	cat >"$PREFIX/bin/$name" <<WRAPPER
#!/data/data/com.termux/files/usr/bin/sh
exec "$INSTALL_DIR/scan-command" "\$@"
WRAPPER
	chmod +x "$PREFIX/bin/$name"
done

echo "Done! Run: Cloudflare-Scanner  (aliases: cloudflare-scanner, scan)"
echo "Commands: Cloudflare-Scanner help | update | restart | uninstall"
