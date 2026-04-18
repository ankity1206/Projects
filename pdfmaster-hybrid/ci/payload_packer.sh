#!/usr/bin/env bash
# ci/payload_packer.sh
#
# Appends the pdfmaster + libs payload to the launcher binary.
# This creates the final self-extracting single-file binary.
#
# Usage:
#   ./ci/payload_packer.sh \
#       --launcher  build/pdfmaster-launcher \
#       --payload   build/payload/           \
#       --output    dist/pdfmaster
#
# The format appended to the launcher:
#   [tar.gz of payload/]  [magic 18 bytes]  [uint64 size LE 8 bytes]
#
# The launcher's selfextract package knows how to find and read this.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ── Defaults ──────────────────────────────────────────────────────────
LAUNCHER=""
PAYLOAD_DIR=""
OUTPUT=""
SIGN_KEY=""   # optional: GPG key ID for signing

# ── Parse args ────────────────────────────────────────────────────────
while [[ $# -gt 0 ]]; do
    case "$1" in
        --launcher)  LAUNCHER="$2";    shift 2 ;;
        --payload)   PAYLOAD_DIR="$2"; shift 2 ;;
        --output)    OUTPUT="$2";      shift 2 ;;
        --sign-key)  SIGN_KEY="$2";    shift 2 ;;
        *) echo "Unknown argument: $1"; exit 1 ;;
    esac
done

[[ -z "$LAUNCHER"    ]] && { echo "Error: --launcher required"; exit 1; }
[[ -z "$PAYLOAD_DIR" ]] && { echo "Error: --payload required";  exit 1; }
[[ -z "$OUTPUT"      ]] && OUTPUT="$LAUNCHER-packed"

MAGIC="PDFMASTER_PAYLOAD\x00"
MAGIC_LEN=18

echo "[packer] Launcher  : $LAUNCHER"
echo "[packer] Payload   : $PAYLOAD_DIR"
echo "[packer] Output    : $OUTPUT"

# ── Create payload tar.gz ─────────────────────────────────────────────
TMPDIR_PACK=$(mktemp -d)
trap 'rm -rf "$TMPDIR_PACK"' EXIT

PAYLOAD_TAR="$TMPDIR_PACK/payload.tar.gz"
echo "[packer] Creating payload archive..."
tar -czf "$PAYLOAD_TAR" -C "$PAYLOAD_DIR" .

PAYLOAD_SIZE=$(stat -c%s "$PAYLOAD_TAR" 2>/dev/null || stat -f%z "$PAYLOAD_TAR")
echo "[packer] Payload size: $(numfmt --to=iec $PAYLOAD_SIZE 2>/dev/null || echo ${PAYLOAD_SIZE}B)"

# ── Assemble output ────────────────────────────────────────────────────
echo "[packer] Assembling self-extracting binary..."
cp "$LAUNCHER" "$OUTPUT"
chmod +x "$OUTPUT"

# Append: tar.gz + magic marker + uint64 size (little-endian)
cat "$PAYLOAD_TAR" >> "$OUTPUT"

# Write magic (18 bytes)
printf '%b' "$MAGIC" >> "$OUTPUT"

# Write uint64 payload size in little-endian (using Python for portability)
python3 -c "
import struct, sys
size = $PAYLOAD_SIZE
sys.stdout.buffer.write(struct.pack('<Q', size))
" >> "$OUTPUT"

FINAL_SIZE=$(stat -c%s "$OUTPUT" 2>/dev/null || stat -f%z "$OUTPUT")
echo "[packer] Final binary: $(numfmt --to=iec $FINAL_SIZE 2>/dev/null || echo ${FINAL_SIZE}B)"

# ── Generate checksums ────────────────────────────────────────────────
echo "[packer] Computing checksums..."
CHECKSUM_FILE="$OUTPUT.sha256"
sha256sum "$OUTPUT" > "$CHECKSUM_FILE"
echo "[packer] SHA256: $(cut -d' ' -f1 "$CHECKSUM_FILE")"

# ── Optional GPG signing ──────────────────────────────────────────────
if [[ -n "$SIGN_KEY" ]]; then
    echo "[packer] Signing with key: $SIGN_KEY"
    gpg --default-key "$SIGN_KEY" \
        --armor --detach-sign \
        --output "$OUTPUT.asc" \
        "$OUTPUT"
    echo "[packer] Signature: $OUTPUT.asc"
fi

echo "[packer] Done: $OUTPUT"
