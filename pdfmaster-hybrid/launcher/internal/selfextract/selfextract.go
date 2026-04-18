// launcher/internal/selfextract/selfextract.go
//
// Manages the embedded binary payload.
//
// Build-time: the release CI pipeline appends a tar.gz payload to the
// launcher binary, preceded by a 16-byte magic marker and 8-byte size:
//
//   [launcher ELF/PE]  [PDFMASTER_PAYLOAD\x00] [uint64 size LE] [tar.gz data]
//
// At runtime we seek to the end of the file, scan backwards for the
// magic, and extract the tar.gz to the install directory.
//
// This "self-extracting binary" technique works on both Linux and Windows:
// - ELF ignores trailing data after the last segment
// - PE ignores data after the certificate table (for unsigned binaries)
//
// Alternative approach for AppImage: the AppImage runtime handles this
// differently (squashfs offset), so this code is only used for the
// standalone .exe and the Linux self-extracting binary.

package selfextract

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	magic     = "PDFMASTER_PAYLOAD\x00" // 18 bytes
	magicLen  = 18
	sizeLen   = 8 // uint64 little-endian
	headerLen = magicLen + sizeLen
)

// ErrNoPayload means this binary has no embedded payload (development mode).
var ErrNoPayload = errors.New("no embedded payload found")

// ChecksumManifest is embedded as checksums.json inside the payload tar.gz.
type ChecksumManifest struct {
	Files map[string]string `json:"files"` // relative path → sha256 hex
}

// HasPayload returns true if this binary has an embedded payload.
func HasPayload() bool {
	_, _, err := findPayloadOffset()
	return err == nil
}

// Extract extracts the embedded tar.gz payload into destDir.
// progressCb is called with (bytesRead, totalBytes).
func Extract(destDir string, progressCb func(read, total int64)) error {
	offset, size, err := findPayloadOffset()
	if err != nil {
		return fmt.Errorf("no payload: %w", err)
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}
	f, err := os.Open(self)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return err
	}

	limitedReader := &progressReader{
		r:        io.LimitReader(f, size),
		total:    size,
		callback: progressCb,
	}

	gr, err := gzip.NewReader(limitedReader)
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar: %w", err)
		}

		destPath := filepath.Join(destDir, filepath.Clean(hdr.Name))

		// Security: prevent path traversal
		if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, os.FileMode(hdr.Mode)|0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return err
			}
			outFile, err := os.OpenFile(destPath,
				os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
				os.FileMode(hdr.Mode)|0o644)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		case tar.TypeSymlink:
			_ = os.Remove(destPath)
			if err := os.Symlink(hdr.Linkname, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// VerifyChecksums reads checksums.json from destDir and validates all files.
func VerifyChecksums(destDir string) error {
	manifestPath := filepath.Join(destDir, "checksums.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// No manifest — skip verification (development mode)
		return nil
	}

	var manifest ChecksumManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("corrupt checksum manifest: %w", err)
	}

	for relPath, expected := range manifest.Files {
		fullPath := filepath.Join(destDir, relPath)
		actual, err := sha256File(fullPath)
		if err != nil {
			return fmt.Errorf("cannot read %s: %w", relPath, err)
		}
		if actual != expected {
			return fmt.Errorf("checksum mismatch for %s\n  expected: %s\n  got:      %s",
				relPath, expected, actual)
		}
	}
	return nil
}

// ── Internal helpers ──────────────────────────────────────────────────

func findPayloadOffset() (offset int64, size int64, err error) {
	self, err := os.Executable()
	if err != nil {
		return 0, 0, err
	}
	f, err := os.Open(self)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()

	fileSize, err := f.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, 0, err
	}
	if fileSize < int64(headerLen) {
		return 0, 0, ErrNoPayload
	}

	// Scan backwards for magic (max 4 KB from end to be fast)
	scanStart := fileSize - int64(headerLen) - 4096
	if scanStart < 0 {
		scanStart = 0
	}

	buf := make([]byte, fileSize-scanStart)
	if _, err := f.Seek(scanStart, io.SeekStart); err != nil {
		return 0, 0, err
	}
	if _, err := io.ReadFull(f, buf); err != nil {
		return 0, 0, err
	}

	// Search for magic bytes
	needle := []byte(magic)
	for i := len(buf) - len(needle); i >= 0; i-- {
		if string(buf[i:i+len(needle)]) == magic {
			sizeOffset := i + magicLen
			if sizeOffset+sizeLen > len(buf) {
				continue
			}
			payloadSize := int64(binary.LittleEndian.Uint64(buf[sizeOffset:]))
			payloadOffset := scanStart + int64(i) + int64(headerLen)
			if payloadOffset+payloadSize <= fileSize {
				return payloadOffset, payloadSize, nil
			}
		}
	}
	return 0, 0, ErrNoPayload
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type progressReader struct {
	r        io.Reader
	read     int64
	total    int64
	callback func(int64, int64)
}

func (p *progressReader) Read(buf []byte) (int, error) {
	n, err := p.r.Read(buf)
	p.read += int64(n)
	if p.callback != nil {
		p.callback(p.read, p.total)
	}
	return n, err
}
