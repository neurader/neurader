package system

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

/* ========================================================================
   v2 SELF-UPDATE ENGINE
   This logic allows the Jumpbox to pull the latest compiled binary from 
   your central developer server.
   ======================================================================== */

// The URL where your Developer Server hosts the latest compiled binary.
// Replace with your actual server IP or domain.
const UpdateURL = "https://neurader.operman.in/releases/neurader-main"

// FetchAndUpgradeJumpbox downloads the new binary from your dev server.
func FetchAndUpgradeJumpbox() error {
	fmt.Printf("[*] neurader v2: Checking for updates from %s...\n", UpdateURL)

	// 1. Initiate the download
	resp, err := http.Get(UpdateURL)
	if err != nil {
		return fmt.Errorf("could not connect to update server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned error: %s", resp.Status)
	}

	// 2. Prepare a temporary path for the download
	// We download to .tmp first to avoid "text file busy" errors 
	// and to ensure we don't break the current running binary.
	destPath := "/usr/local/bin/neurader"
	tempPath := destPath + ".tmp"

	out, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}

	// 3. Stream the data from the server to the temp file
	_, err = io.Copy(out, resp.Body)
	out.Close() // Must close before renaming
	if err != nil {
		return fmt.Errorf("failed during download stream: %v", err)
	}

	// 4. Atomic Swap
	// os.Rename is atomic in Linux, meaning the old binary is replaced 
	// by the new one instantly. This DOES NOT touch /etc/neurader/ or hosts.yml.
	err = os.Rename(tempPath, destPath)
	if err != nil {
		return fmt.Errorf("failed to swap binary: %v. (Are you running with sudo?)", err)
	}

	fmt.Println("[+] Jumpbox successfully upgraded. Local state and configuration preserved.")
	return nil
}
