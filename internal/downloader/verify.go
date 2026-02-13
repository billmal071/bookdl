package downloader

import (
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/billmal071/bookdl/internal/db"
)

// VerifyChecksum verifies the MD5 checksum of a downloaded file
func VerifyChecksum(download *db.Download) error {
	if download.FilePath == "" {
		return fmt.Errorf("file path is empty")
	}

	// Open the file
	file, err := os.Open(download.FilePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Calculate MD5 hash
	hash := md5.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}

	// Get checksum as hex string
	checksum := fmt.Sprintf("%x", hash.Sum(nil))

	// Compare with expected hash
	expectedHash := strings.ToLower(strings.TrimSpace(download.MD5Hash))
	if checksum != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, checksum)
	}

	return nil
}

// VerifyAndMark verifies a download and updates its verified status
func VerifyAndMark(download *db.Download) error {
	err := VerifyChecksum(download)
	if err != nil {
		// Mark as not verified
		if markErr := db.MarkVerified(download.ID, false); markErr != nil {
			return fmt.Errorf("verification failed (%v) and failed to update status: %w", err, markErr)
		}
		return err
	}

	// Mark as verified
	if err := db.MarkVerified(download.ID, true); err != nil {
		return fmt.Errorf("verification succeeded but failed to update status: %w", err)
	}

	return nil
}
