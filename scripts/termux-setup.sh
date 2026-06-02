#!/data/data/com.termux/files/usr/bin/sh
# Cloudflare Scanner — one-liner install for Termux
# Run: curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh

PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"
DIR="$HOME/Cloudflare-Scanner"

pkg update -y && pkg install -y curl

mkdir -p "$DIR"
cd "$DIR" || exit 1

echo "Downloading latest release..."
TAG=$(curl -s https://api.github.com/repos/QMahyar/Cloudflare-Scanner/releases/latest \
  | grep '"tag_name"' | cut -d'"' -f4)
URL="https://github.com/QMahyar/Cloudflare-Scanner/releases/download/$TAG/Cloudflare-Scanner-${TAG}-termux-arm64.tar.gz"
curl -sL "$URL" | gunzip -c | tar -xf -

chmod +x Cloudflare-Scanner xray

echo '#!/data/data/com.termux/files/usr/bin/sh
exec ~/Cloudflare-Scanner/Cloudflare-Scanner "$@"' > "$PREFIX/bin/scan"
chmod +x "$PREFIX/bin/scan"

echo "Done! Type 'scan' to run."
