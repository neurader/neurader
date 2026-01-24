package system

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

// UpdateBinary downloads a new version and replaces the executable only.
// It DOES NOT touch /etc/sentinex/ where your hosts.yml lives.
func UpdateBinary(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 1. Download to a temporary file first
	tempPath := "/usr/local/bin/sentinex.tmp"
	out, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	// 2. Replace the old binary with the new one
	// Renaming is an atomic operation in Linux
	err = os.Rename(tempPath, "/usr/local/bin/sentinex")
	if err != nil {
		return fmt.Errorf("could not replace binary: %v", err)
	}

	return nil
}
