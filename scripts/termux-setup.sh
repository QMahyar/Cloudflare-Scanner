#!/data/data/com.termux/files/usr/bin/sh
# Cloudflare Scanner — one-liner install for Termux
# Run: curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh

PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"

pkg install -y curl jq 2>/dev/null

DIR="$HOME/Cloudflare-Scanner"
mkdir -p "$DIR"
cd "$DIR" || exit 1

echo "Downloading latest release..."
URL=$(curl -s https://api.github.com/repos/QMahyar/Cloudflare-Scanner/releases/latest \
  | jq -r '.assets[] | select(.name | test("termux-arm64")) | .browser_download_url')

if [ -z "$URL" ]; then
  echo "Error: could not find termux-arm64 release asset"
  exit 1
fi

curl -sL "$URL" | tar -xzf -
chmod +x Cloudflare-Scanner xray

echo '#!/data/data/com.termux/files/usr/bin/sh
exec ~/Cloudflare-Scanner/Cloudflare-Scanner "$@"' > "$PREFIX/bin/scan"
chmod +x "$PREFIX/bin/scan"

echo "Done! Type 'scan' to run."
