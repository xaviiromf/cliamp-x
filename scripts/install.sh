#!/bin/sh
# cliamp-x installer for Linux.
#
#   ./scripts/install.sh              build from this checkout and install
#   ./scripts/install.sh --release    install the latest release binary
#   ./scripts/install.sh --uninstall  remove cliamp-x (keeps your config)
#
# Options:
#   --prefix DIR   install under DIR (default: ~/.local, or /usr/local as root)
#   --no-deps      do not install packages (yt-dlp, ffmpeg, ALSA, build tools)
#   --yes          do not ask before installing packages
#
# Supported package managers: apt (Debian, Ubuntu, Mint, Pop!_OS), dnf/yum
# (Fedora, RHEL, Rocky, Alma), zypper (openSUSE), pacman (Arch, Manjaro,
# EndeavourOS, Omarchy), apk (Alpine), xbps-install (Void), emerge (Gentoo).
set -eu

REPO="xaviiromf/cliamp-x"
BINARY="cliamp-x"
MODE="source"
INSTALL_DEPS=1
ASSUME_YES=0
PREFIX=""

say() { printf '%s\n' "$*"; }
warn() { printf 'cliamp-x: %s\n' "$*" >&2; }
die() { warn "$*"; exit 1; }

while [ $# -gt 0 ]; do
    case "$1" in
        --release) MODE="release" ;;
        --source) MODE="source" ;;
        --uninstall) MODE="uninstall" ;;
        --no-deps) INSTALL_DEPS=0 ;;
        --yes|-y) ASSUME_YES=1 ;;
        --prefix) shift; [ $# -gt 0 ] || die "--prefix needs a directory"; PREFIX="$1" ;;
        --prefix=*) PREFIX="${1#--prefix=}" ;;
        -h|--help) sed -n '2,15p' "$0"; exit 0 ;;
        *) die "unknown option: $1 (see --help)" ;;
    esac
    shift
done

[ "$(uname -s)" = "Linux" ] || die "cliamp-x targets Linux only"

if [ -z "$PREFIX" ]; then
    if [ "$(id -u)" -eq 0 ]; then PREFIX="/usr/local"; else PREFIX="$HOME/.local"; fi
fi
BIN_DIR="$PREFIX/bin"
SHARE_DIR="$PREFIX/share"

# SUDO runs package installs: nothing as root, sudo or doas otherwise.
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then SUDO="sudo"
    elif command -v doas >/dev/null 2>&1; then SUDO="doas"
    fi
fi

# REPO_DIR is the checkout this script came from, if any.
SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
REPO_DIR=$(CDPATH='' cd -- "$SCRIPT_DIR/.." && pwd)

if [ "$MODE" = "uninstall" ]; then
    rm -f "$BIN_DIR/$BINARY" \
        "$SHARE_DIR/applications/cliamp-x.desktop" \
        "$SHARE_DIR/icons/hicolor/scalable/apps/cliamp-x.svg"
    say "Removed cliamp-x from $PREFIX. Your settings stay in ~/.config/cliamp-x."
    exit 0
fi

# --- packages -------------------------------------------------------------

detect_pm() {
    for pm in apt-get dnf yum zypper pacman apk xbps-install emerge; do
        if command -v "$pm" >/dev/null 2>&1; then
            echo "$pm"
            return
        fi
    done
    echo ""
}

PM=$(detect_pm)

# uses_pipewire reports whether the sound server is PipeWire, which needs
# its ALSA plugin for the ALSA output of cliamp-x.
uses_pipewire() {
    command -v pw-cli >/dev/null 2>&1 || pgrep -x pipewire >/dev/null 2>&1
}

packages_for() {
    # $1 = runtime | build
    case "$PM:$1" in
        apt-get:runtime) echo "yt-dlp ffmpeg libasound2 ca-certificates"; if uses_pipewire; then echo "pipewire-alsa"; fi ;;
        apt-get:build) echo "golang-go gcc make git pkg-config libasound2-dev" ;;
        dnf:runtime|yum:runtime) echo "yt-dlp ffmpeg-free alsa-lib"; if uses_pipewire; then echo "pipewire-alsa"; fi ;;
        dnf:build|yum:build) echo "golang gcc make git pkgconf-pkg-config alsa-lib-devel" ;;
        zypper:runtime) echo "yt-dlp ffmpeg alsa"; if uses_pipewire; then echo "pipewire-alsa"; fi ;;
        zypper:build) echo "go gcc make git pkg-config alsa-devel" ;;
        pacman:runtime) echo "yt-dlp ffmpeg alsa-lib"; if uses_pipewire; then echo "pipewire-alsa"; fi ;;
        pacman:build) echo "go gcc make git pkgconf" ;;
        apk:runtime) echo "yt-dlp ffmpeg alsa-lib"; if uses_pipewire; then echo "pipewire-alsa"; fi ;;
        apk:build) echo "go gcc musl-dev make git pkgconf alsa-lib-dev" ;;
        xbps-install:runtime) echo "yt-dlp ffmpeg alsa-lib"; if uses_pipewire; then echo "alsa-pipewire"; fi ;;
        xbps-install:build) echo "go gcc make git pkg-config alsa-lib-devel" ;;
        emerge:runtime) echo "net-misc/yt-dlp media-video/ffmpeg media-libs/alsa-lib" ;;
        emerge:build) echo "dev-lang/go sys-devel/gcc dev-build/make dev-vcs/git" ;;
    esac
}

install_packages() {
    # $@ = packages
    [ $# -gt 0 ] || return 0
    if [ "$ASSUME_YES" -eq 0 ] && [ -t 0 ]; then
        printf 'Install with %s: %s? [Y/n] ' "$PM" "$*"
        read -r answer || answer=n
        case "$answer" in ""|y|Y|yes|s|S|si) ;; *) warn "skipped the packages"; return 0 ;; esac
    fi
    if [ "$PM" = "apt-get" ]; then $SUDO apt-get update -qq || true; fi
    if pm_install "$@"; then return 0; fi
    # One missing package fails the whole transaction on most managers, so
    # retry one by one and keep what installs.
    missing=""
    for pkg in "$@"; do
        pm_install "$pkg" || missing="$missing $pkg"
    done
    if [ -n "$missing" ]; then warn "could not install:$missing (install them by hand if you need them)"; fi
}

pm_install() {
    case "$PM" in
        apt-get) $SUDO env DEBIAN_FRONTEND=noninteractive apt-get install -y "$@" ;;
        dnf) $SUDO dnf install -y "$@" ;;
        yum) $SUDO yum install -y "$@" ;;
        zypper) $SUDO zypper --non-interactive install "$@" ;;
        pacman) $SUDO pacman -S --needed --noconfirm "$@" ;;
        apk) $SUDO apk add "$@" ;;
        xbps-install) $SUDO xbps-install -Sy "$@" ;;
        emerge) $SUDO emerge --noreplace "$@" ;;
        *) return 1 ;;
    esac
}

ensure_runtime() {
    if [ "$INSTALL_DEPS" -eq 0 ]; then return; fi
    if [ -z "$PM" ]; then
        warn "no known package manager: install yt-dlp, ffmpeg and alsa-lib yourself"
        return
    fi
    # shellcheck disable=SC2046
    install_packages $(packages_for runtime)
    if ! command -v yt-dlp >/dev/null 2>&1; then
        warn "yt-dlp is missing; trying pipx/pip"
        if command -v pipx >/dev/null 2>&1; then pipx install yt-dlp || true
        elif command -v pip3 >/dev/null 2>&1; then pip3 install --user yt-dlp || true
        fi
    fi
}

ensure_build_tools() {
    if [ "$INSTALL_DEPS" -eq 1 ] && [ -n "$PM" ]; then
        # shellcheck disable=SC2046
        install_packages $(packages_for build)
    fi
    if ! go_recent; then
        download_go
    fi
    command -v gcc >/dev/null 2>&1 || command -v cc >/dev/null 2>&1 || die "a C compiler (gcc) is required for the ALSA output"
}

# go_recent reports whether the installed Go can fetch the toolchain go.mod
# asks for (Go 1.21 or newer).
go_recent() {
    command -v go >/dev/null 2>&1 || return 1
    minor=$(go env GOVERSION 2>/dev/null | sed -n 's/^go1\.\([0-9]*\).*/\1/p')
    [ -n "$minor" ] && [ "$minor" -ge 21 ]
}

# download_go fetches the Go release that go.mod names into a temporary
# directory and puts it first in PATH, for distributions with an old Go.
download_go() {
    version=$(sed -n 's/^go \([0-9.]*\)$/\1/p' "$REPO_DIR/go.mod")
    case "$(uname -m)" in
        x86_64|amd64) goarch="amd64" ;;
        aarch64|arm64) goarch="arm64" ;;
        *) die "install Go 1.21 or newer for $(uname -m): https://go.dev/dl/" ;;
    esac
    GO_TMP=$(mktemp -d)
    say "Downloading Go $version..."
    fetch "https://go.dev/dl/go$version.linux-$goarch.tar.gz" "$GO_TMP/go.tgz"
    tar -C "$GO_TMP" -xzf "$GO_TMP/go.tgz"
    PATH="$GO_TMP/go/bin:$PATH"
    export PATH
}

# --- install ----------------------------------------------------------------

install_extras() {
    # $1 = directory with cliamp-x.desktop and Cliamp.svg
    src="$1"
    if [ -f "$src/cliamp-x.desktop" ]; then
        mkdir -p "$SHARE_DIR/applications"
        sed "s#^Exec=cliamp-x#Exec=$BIN_DIR/cliamp-x#" "$src/cliamp-x.desktop" > "$SHARE_DIR/applications/cliamp-x.desktop"
    fi
    if [ -f "$src/Cliamp.svg" ]; then
        mkdir -p "$SHARE_DIR/icons/hicolor/scalable/apps"
        cp "$src/Cliamp.svg" "$SHARE_DIR/icons/hicolor/scalable/apps/cliamp-x.svg"
    fi
    if command -v update-desktop-database >/dev/null 2>&1; then
        update-desktop-database "$SHARE_DIR/applications" >/dev/null 2>&1 || true
    fi
}

build_from_source() {
    [ -f "$REPO_DIR/go.mod" ] || die "run this script from a cliamp-x checkout, or use --release"
    ensure_build_tools
    say "Building cliamp-x..."
    # GOTOOLCHAIN=auto lets an older Go fetch the version go.mod asks for.
    (cd "$REPO_DIR" && GOTOOLCHAIN=auto make build)
    mkdir -p "$BIN_DIR"
    install -m 755 "$REPO_DIR/$BINARY" "$BIN_DIR/$BINARY"
    install_extras "$REPO_DIR"
    if [ -n "${GO_TMP:-}" ]; then rm -rf "$GO_TMP"; fi
}

fetch() {
    # $1 = url, $2 = output file. GITHUB_TOKEN reaches a private repository.
    if command -v curl >/dev/null 2>&1; then
        if [ -n "${GITHUB_TOKEN:-}" ]; then
            curl -fsSL -H "Authorization: Bearer $GITHUB_TOKEN" -o "$2" "$1"
        else
            curl -fsSL -o "$2" "$1"
        fi
    elif command -v wget >/dev/null 2>&1; then
        if [ -n "${GITHUB_TOKEN:-}" ]; then
            wget -q --header="Authorization: Bearer $GITHUB_TOKEN" -O "$2" "$1"
        else
            wget -q -O "$2" "$1"
        fi
    else
        die "curl or wget is required"
    fi
}

install_release() {
    case "$(uname -m)" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) die "no release build for $(uname -m); build from source instead" ;;
    esac
    asset="cliamp-x-linux-$arch.tar.gz"
    tmp=$(mktemp -d)
    trap 'rm -rf "$tmp"' EXIT HUP INT TERM
    say "Downloading $asset..."
    if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
        # gh also reaches a private repository.
        gh release download --repo "$REPO" --pattern "$asset" --pattern checksums.txt --dir "$tmp"
    else
        fetch "https://github.com/$REPO/releases/latest/download/$asset" "$tmp/$asset"
        fetch "https://github.com/$REPO/releases/latest/download/checksums.txt" "$tmp/checksums.txt"
    fi
    (cd "$tmp" && grep " $asset\$" checksums.txt | sha256sum -c -) || die "checksum mismatch for $asset"
    tar -xzf "$tmp/$asset" -C "$tmp"
    mkdir -p "$BIN_DIR"
    install -m 755 "$tmp/cliamp-x/$BINARY" "$BIN_DIR/$BINARY"
    install_extras "$tmp/cliamp-x"
}

ensure_runtime
case "$MODE" in
    source) build_from_source ;;
    release) install_release ;;
esac

say ""
say "cliamp-x is installed in $BIN_DIR/$BINARY."
case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) say "Add $BIN_DIR to your PATH, for example: echo 'export PATH=\"$BIN_DIR:\$PATH\"' >> ~/.profile" ;;
esac
say ""
say "Next steps:"
say "  1. Sign in at https://music.youtube.com in your browser."
say "  2. Run: cliamp-x login     (reads the sign-in from your browser once)"
say "  3. Run: cliamp-x"
