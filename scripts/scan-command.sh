#!/usr/bin/env sh
# Cloudflare Scanner command manager. Installed as Cloudflare-Scanner,
# cloudflare-scanner, and scan by the platform installers.
set -e

REPO="QMahyar/Cloudflare-Scanner"
INSTALL_DIR="$HOME/.local/share/cloudflare-scanner"
APP="$INSTALL_DIR/Cloudflare-Scanner"
WRAPPER_DIR=$(dirname "$0")

stop_app() {
	# Match only the installed binary, not unrelated scanner processes.
	pkill -f "$APP" 2>/dev/null || true
	sleep 1
}

help() {
	cat <<'HELP'
Cloudflare Scanner

Usage: Cloudflare-Scanner [command]
       scan [command]

Commands:
  start       Launch the app (default)
  restart     Stop the installed app and launch it again
  update      Install the latest GitHub release
  uninstall   Remove the app and command aliases
  help        Show this help
HELP
}

command=${1:-start}
case "$command" in
start | launch | run)
	exec "$APP"
	;;
restart)
	stop_app
	exec "$APP"
	;;
update)
	stop_app
	case "$(uname -s)" in
	Darwin) installer="install-macos.sh" ;;
	Linux)
		if [ -n "${TERMUX_VERSION:-}" ] || [ -d /data/data/com.termux ]; then
			installer="termux-setup.sh"
		else
			installer="install-linux.sh"
		fi
		;;
	*) echo "Unsupported platform." >&2; exit 1 ;;
	esac
	curl -fsSL "https://raw.githubusercontent.com/${REPO}/master/scripts/$installer" | sh
	;;
uninstall | remove)
	stop_app
	rm -rf "$INSTALL_DIR"
	for path in "$WRAPPER_DIR/Cloudflare-Scanner" "$WRAPPER_DIR/cloudflare-scanner" "$WRAPPER_DIR/scan" "$HOME/.local/bin/Cloudflare-Scanner" "$HOME/.local/bin/cloudflare-scanner" "$HOME/.local/bin/scan" "$HOME/bin/Cloudflare-Scanner" "$HOME/bin/cloudflare-scanner" "$HOME/bin/scan"; do
		rm -f "$path"
	done
	if [ -d /data/data/com.termux ]; then
		rm -f "${PREFIX:-/data/data/com.termux/files/usr}/bin/Cloudflare-Scanner" \
			"${PREFIX:-/data/data/com.termux/files/usr}/bin/cloudflare-scanner" \
			"${PREFIX:-/data/data/com.termux/files/usr}/bin/scan"
	fi
	if [ -e /usr/local/bin/Cloudflare-Scanner ] || [ -e /usr/local/bin/cloudflare-scanner ] || [ -e /usr/local/bin/scan ]; then
		sudo rm -f /usr/local/bin/Cloudflare-Scanner /usr/local/bin/cloudflare-scanner /usr/local/bin/scan
	fi
	echo "Cloudflare Scanner uninstalled."
	;;
help | -h | --help)
	help
	;;
*)
	echo "Unknown command: $command" >&2
	help >&2
	exit 2
	;;
esac
