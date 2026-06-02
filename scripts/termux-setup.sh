#!/data/data/com.termux/files/usr/bin/sh
# Cloudflare Scanner — Termux setup
# Run: curl -sL https://raw.githubusercontent.com/QMahyar/Cloudflare-Scanner/master/scripts/termux-setup.sh | sh
set -e
DIR="$HOME/Cloudflare-Scanner"
mkdir -p "$DIR" && cd "$DIR"
echo "Downloading latest release..."
curl -s https://api.github.com/repos/QMahyar/Cloudflare-Scanner/releases/latest \
  | grep "browser_download_url.*termux-arm64" \
  | cut -d'"' -f4 \
  | xargs curl -L \
  | tar -xzf -
chmod +x Cloudflare-Scanner xray
echo '#!/data/data/com.termux/files/usr/bin/sh
exec ~/Cloudflare-Scanner/Cloudflare-Scanner "$@"' > "$PREFIX/bin/scan"
chmod +x "$PREFIX/bin/scan"
echo "Installed! Type 'scan' to run."
