#!/usr/bin/env bash
# Cloudflare Scanner — local build script for Linux / macOS / Termux
#
# Builds the app from source and bundles the matching xray-core sidecar,
# producing release-identical archives under ./dist/.
#
# Usage:
#   ./scripts/build.sh                     # build for the current host platform
#   ./scripts/build.sh all                 # build every supported platform
#   ./scripts/build.sh linux-amd64         # build one specific platform
#   ./scripts/build.sh linux-amd64 darwin-arm64   # build several
#
# Supported platform keys:
#   windows-amd64  windows-arm64  linux-amd64  linux-arm64
#   termux-arm64   darwin-amd64   darwin-arm64
#
# Environment overrides:
#   VERSION=v3.0.1     # version string baked into the binary (default: git describe or "dev")
#   XRAY_VERSION=...   # xray-core release tag to bundle (default: v1.8.24)
#   NO_XRAY=1          # skip downloading xray (build the binary only)
#   NO_ARCHIVE=1       # leave loose files in dist/<platform>/, skip .zip/.tar.gz
#   GO_VERSION=1.26.2  # Go version to auto-install if Go is missing

set -eu

# ── Config ──────────────────────────────────────────────────────────────────
XRAY_VERSION="${XRAY_VERSION:-v1.8.24}"
GO_VERSION="${GO_VERSION:-1.26.2}"
APP="Cloudflare-Scanner"

# Resolve the repo root from the script's own location — robust to the current
# working directory, symlinks, and being sourced (./build.sh, `. build.sh`, a
# symlink on $PATH, or invocation from any directory all resolve the same).
_src="${BASH_SOURCE[0]:-$0}"
while [ -h "$_src" ]; do
  _dir="$(cd -P "$(dirname "$_src")" >/dev/null 2>&1 && pwd)"
  _src="$(readlink "$_src")"
  case "$_src" in /*) ;; *) _src="$_dir/$_src" ;; esac
done
SCRIPT_DIR="$(cd -P "$(dirname "$_src")" >/dev/null 2>&1 && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
DIST="$REPO_ROOT/dist"
cd "$REPO_ROOT"

# ── Pretty logging ──────────────────────────────────────────────────────────
if [ -t 1 ]; then
  B="\033[1m"; G="\033[32m"; Y="\033[33m"; R="\033[31m"; D="\033[2m"; N="\033[0m"
else
  B=""; G=""; Y=""; R=""; D=""; N=""
fi
log()  { printf "${B}==>${N} %s\n" "$*"; }
ok()   { printf "${G}  ok${N} %s\n" "$*"; }
warn() { printf "${Y}  ! ${N} %s\n" "$*"; }
die()  { printf "${R}error:${N} %s\n" "$*" >&2; exit 1; }

# ── Platform matrix (mirrors .github/workflows/release.yml) ──────────────────
# key|GOOS|GOARCH|ext|xray_in|xray_zip
matrix() {
  cat <<'EOF'
windows-amd64|windows|amd64|.exe|xray.exe|Xray-windows-64.zip
windows-arm64|windows|arm64|.exe|xray.exe|Xray-windows-arm64-v8a.zip
linux-amd64|linux|amd64||xray|Xray-linux-64.zip
linux-arm64|linux|arm64||xray|Xray-linux-arm64-v8a.zip
termux-arm64|linux|arm64||xray|Xray-android-arm64-v8a.zip
darwin-amd64|darwin|amd64||xray|Xray-macos-64.zip
darwin-arm64|darwin|arm64||xray|Xray-macos-arm64-v8a.zip
EOF
}

row_for() {
  matrix | grep "^$1|" || true
}

# ── Detect the current host platform key ────────────────────────────────────
detect_host() {
  os="$(uname -s)"; arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) a=amd64 ;;
    aarch64|arm64) a=arm64 ;;
    *) die "unsupported host architecture: $arch" ;;
  esac
  case "$os" in
    Linux)
      if [ -n "${TERMUX_VERSION:-}" ] || [ -d /data/data/com.termux ]; then
        echo "termux-arm64"
      else
        echo "linux-$a"
      fi ;;
    Darwin) echo "darwin-$a" ;;
    *) die "unsupported host OS: $os (use an explicit platform key)" ;;
  esac
}

# ── Ensure a Go toolchain (>= go.mod requirement) is available ──────────────
GO=""
need_go="$(grep -E '^go [0-9]' "$REPO_ROOT/go.mod" | awk '{print $2}')"

version_ge() {
  # version_ge A B  → 0 (true) if A >= B
  [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n1)" = "$2" ]
}

ensure_go() {
  if command -v go >/dev/null 2>&1; then
    have="$(go version | awk '{print $3}' | sed 's/^go//')"
    if version_ge "$have" "$need_go"; then
      GO="$(command -v go)"
      ok "Go $have (>= $need_go required)"
      return
    fi
    warn "Go $have is older than required $need_go — installing a local copy"
  else
    warn "Go not found — installing a local copy (Go $GO_VERSION)"
  fi

  # Auto-install Go into a cache dir without touching system paths.
  hostkey="$(detect_host)"
  case "$hostkey" in
    termux-arm64)
      if command -v pkg >/dev/null 2>&1; then
        log "Installing Go via Termux pkg"
        pkg install -y golang
        GO="$(command -v go)"; ok "Go installed via pkg"; return
      fi ;;
  esac

  goos="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$(uname -m)" in
    x86_64|amd64) goarch=amd64 ;;
    aarch64|arm64) goarch=arm64 ;;
    *) die "cannot auto-install Go for arch $(uname -m)" ;;
  esac
  cache="$REPO_ROOT/.gobuild"
  mkdir -p "$cache"
  tarball="go${GO_VERSION}.${goos}-${goarch}.tar.gz"
  url="https://go.dev/dl/${tarball}"
  log "Downloading $url"
  if command -v curl >/dev/null 2>&1; then
    curl -fSL "$url" -o "$cache/$tarball"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "$cache/$tarball" "$url"
  else
    die "need curl or wget to download Go"
  fi
  rm -rf "$cache/go"
  tar -xzf "$cache/$tarball" -C "$cache"
  GO="$cache/go/bin/go"
  [ -x "$GO" ] || die "Go install failed"
  ok "Go $GO_VERSION installed to $cache/go"
}

# ── Download + extract the xray-core binary for one platform ────────────────
fetch_xray() {
  outdir="$1"; xray_in="$2"; xray_zip="$3"
  [ -n "${NO_XRAY:-}" ] && { warn "NO_XRAY set — skipping xray for $outdir"; return; }
  url="https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/${xray_zip}"
  tmp="$(mktemp -d)"
  log "Fetching xray-core ${XRAY_VERSION} ($xray_zip)"
  if command -v curl >/dev/null 2>&1; then
    curl -fSL "$url" -o "$tmp/xray.zip"
  else
    wget -O "$tmp/xray.zip" "$url"
  fi
  command -v unzip >/dev/null 2>&1 || die "unzip is required to extract xray-core"
  unzip -o -q "$tmp/xray.zip" "$xray_in" -d "$outdir"
  chmod +x "$outdir/$xray_in" 2>/dev/null || true
  rm -rf "$tmp"
  ok "xray-core → $outdir/$xray_in"
}

# ── Build one platform ──────────────────────────────────────────────────────
build_one() {
  key="$1"
  row="$(row_for "$key")"
  [ -n "$row" ] || die "unknown platform: $key (run with no args to see host, or 'all')"

  IFS='|' read -r _ goos goarch ext xray_in xray_zip <<EOF
$row
EOF

  outdir="$DIST/$key"
  rm -rf "$outdir"; mkdir -p "$outdir"
  binname="${APP}${ext}"

  log "Building $key  (GOOS=$goos GOARCH=$goarch, version=$VERSION)"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 "$GO" build \
    -trimpath \
    -ldflags="-s -w -X 'main.Version=${VERSION}'" \
    -o "$outdir/$binname" .
  ok "binary → $outdir/$binname"

  fetch_xray "$outdir" "$xray_in" "$xray_zip"

  if [ -z "${NO_ARCHIVE:-}" ]; then
    ( cd "$outdir"
      if [ "$goos" = "windows" ]; then
        archive="${APP}-${VERSION}-${key}.zip"
        if command -v zip >/dev/null 2>&1; then
          zip -q -j "$DIST/$archive" "$binname" ${NO_XRAY:+} $( [ -z "${NO_XRAY:-}" ] && echo "$xray_in" )
        else
          warn "zip not found — leaving loose files in $outdir"
          archive=""
        fi
      else
        archive="${APP}-${VERSION}-${key}.tar.gz"
        files="$binname"
        [ -z "${NO_XRAY:-}" ] && files="$files $xray_in"
        tar -czf "$DIST/$archive" $files
      fi
      [ -n "${archive:-}" ] && ok "archive → dist/$archive"
    )
  fi
}

# ── Resolve version ─────────────────────────────────────────────────────────
if [ -z "${VERSION:-}" ]; then
  if command -v git >/dev/null 2>&1 && git -C "$REPO_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
    VERSION="$(git -C "$REPO_ROOT" describe --tags --always --dirty 2>/dev/null || echo dev)"
  else
    VERSION="dev"
  fi
fi

# ── Main ────────────────────────────────────────────────────────────────────
log "Cloudflare Scanner build  —  version ${VERSION}, xray ${XRAY_VERSION}"
ensure_go
"$GO" vet ./... && ok "go vet clean"

targets=""
if [ "$#" -eq 0 ]; then
  targets="$(detect_host)"
  log "No target given — building host platform: $targets"
elif [ "$1" = "all" ]; then
  targets="$(matrix | cut -d'|' -f1)"
else
  targets="$*"
fi

mkdir -p "$DIST"
for key in $targets; do
  build_one "$key"
done

log "Done. Artifacts in: $DIST"
ls -1 "$DIST" 2>/dev/null | sed 's/^/  /' || true
