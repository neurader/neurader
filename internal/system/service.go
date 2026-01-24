package system

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
)

/* ========================================================================
   SENTINEX SYSTEM CORE (v2.0.0)
   Handles Service Installation, Updates, and User Management.
   ======================================================================== */

const (
	// Replace with your Developer Server URL for the v2 update push
	UpdateURL         = "http://YOUR_DEV_SERVER_IP/sentinex"
	BinaryDestination = "/usr/local/bin/sentinex"
	ConfigDir         = "/etc/sentinex"
	ServicePath       = "/etc/systemd/system/sentinex.service"
)

/* =========================
   1. UPGRADE LOGIC
========================= */

// FetchAndUpgradeJumpbox pulls the latest compiled binary from the Dev Server.
// It uses an "Atomic Swap" to replace the binary without downtime or config loss.
func FetchAndUpgradeJumpbox() error {
	fmt.Printf("[*] Connecting to Developer Server: %s\n", UpdateURL)

	resp, err := http.Get(UpdateURL)
	if err != nil {
		return fmt.Errorf("failed to reach update server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server error: %s", resp.Status)
	}

	// Step A: Download to a temp file
	tempPath := BinaryDestination + ".tmp"
	out, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("cannot create temp file: %v", err)
	}
	
	_, err = io.Copy(out, resp.Body)
	out.Close() // Close before swapping
	if err != nil {
		return fmt.Errorf("download failed: %v", err)
	}

	// Step B: Atomic Swap (Binary replacement only)
	// This preserves /etc/sentinex/hosts.yml perfectly.
	err = os.Rename(tempPath, BinaryDestination)
	if err != nil {
		return fmt.Errorf("failed to overwrite binary: %v", err)
	}

	fmt.Println("[+] Jumpbox upgraded to latest version locally.")
	return nil
}

/* =========================
   2. INSTALLATION LOGIC
========================= */

// InstallService configures Sentinex as a background systemd daemon.
func InstallService() {
	fmt.Println("[*] Setting up Sentinex as a system service...")

	// Create config directory for inventory/keys
	os.MkdirAll(ConfigDir, 0755)

	// Move binary to system path (/usr/local/bin)
	self, err := os.Executable()
	if err == nil {
		input, _ := os.ReadFile(self)
		os.WriteFile(BinaryDestination, input, 0755)
	}

	// Define systemd unit
	serviceFile := `[Unit]
Description=Sentinex Security Agent
After=network.target

[Service]
ExecStart=/usr/local/bin/sentinex daemon
Restart=always
User=root
WorkingDirectory=/etc/sentinex

[Install]
WantedBy=multi-user.target`

	// Write and activate service
	os.WriteFile(ServicePath, []byte(serviceFile), 0644)
	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "sentinex").Run()
	exec.Command("systemctl", "start", "sentinex").Run()

	fmt.Println("[+] Sentinex service installed and started.")
}

/* =========================
   3. USER MANAGEMENT
========================= */

// CreatesentinexUser prepares the Child nodes with a dedicated automation user.
func CreatesentinexUser() {
	fmt.Println("[*] Preparing 'sentinex' user...")
	
	// Create user with a home directory for SSH keys
	exec.Command("useradd", "-m", "-s", "/bin/bash", "sentinex").Run()
	
	// Prepare .ssh folder
	sshPath := "/home/sentinex/.ssh"
	os.MkdirAll(sshPath, 0700)
	
	// Set ownership
	exec.Command("chown", "-R", "sentinex:sentinex", "/home/sentinex").Run()
}
