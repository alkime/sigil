#!/usr/bin/env sh
# Installs sigil from this extracted archive:
#   1. verifies the binary against sigil.sha256
#   2. strips the macOS quarantine attribute (no-op on Linux)
#   3. moves the binary into ~/.local/bin if that directory exists

set -eu

script_dir="$(cd -- "$(dirname -- "$0")" && pwd)"
binary="$script_dir/sigil"
sidecar="$script_dir/sigil.sha256"

if [ ! -f "$binary" ]; then
    echo "error: sigil binary not found next to this script ($script_dir)" >&2
    exit 1
fi

echo "==> Verifying checksum"
if [ ! -f "$sidecar" ]; then
    echo "warn: sigil.sha256 not found — skipping verification" >&2
elif command -v shasum >/dev/null 2>&1; then
    (cd "$script_dir" && shasum -a 256 -c sigil.sha256)
elif command -v sha256sum >/dev/null 2>&1; then
    (cd "$script_dir" && sha256sum -c sigil.sha256)
else
    echo "warn: neither shasum nor sha256sum found — skipping verification" >&2
fi

if [ "$(uname -s)" = "Darwin" ]; then
    if xattr -p com.apple.quarantine "$binary" >/dev/null 2>&1; then
        echo ""
        echo "macOS marked this binary as quarantined (downloaded from the internet)."
        echo "Without removing that attribute, Gatekeeper will block it on first run."
        if [ -t 0 ]; then
            printf "Remove com.apple.quarantine from sigil? [Y/n] "
            answer=""
            read -r answer || true
            case "$answer" in
                ""|y|Y|yes|YES|Yes)
                    xattr -d com.apple.quarantine "$binary"
                    echo "    removed."
                    ;;
                *)
                    echo "    skipped. Run manually later: xattr -d com.apple.quarantine $binary"
                    ;;
            esac
        else
            echo "warn: non-interactive shell — not removing quarantine."
            echo "      run manually: xattr -d com.apple.quarantine $binary"
        fi
    fi
fi

chmod +x "$binary"

dest="$HOME/.local/bin"
if [ -d "$dest" ]; then
    echo "==> Installing to $dest/sigil"
    mv -f "$binary" "$dest/sigil"
    echo ""
    echo "Done. Verify with: sigil --version"
    case ":${PATH:-}:" in
        *":$dest:"*) ;;
        *)
            echo ""
            echo "Note: $dest is not in your PATH. Add it with:"
            echo "    export PATH=\"$dest:\$PATH\""
            ;;
    esac
else
    echo ""
    echo "$dest does not exist — leaving the binary here:"
    echo "    $binary"
    echo ""
    echo "Move it somewhere in your PATH, e.g.:"
    echo "    mkdir -p \"$dest\" && mv \"$binary\" \"$dest/\" && export PATH=\"$dest:\$PATH\""
fi
