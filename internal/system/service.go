package system

import (
	"fmt"
	"os"
	"os/exec"
)

/* ========================================================================
   neurader SYSTEM SERVICE (v2.0.0)
   Handles Service Installation and User Management.
   ======================================================================== */

const (
	BinaryDestination = "/usr/local/bin/neurader"
	ConfigDir         = "/etc/neurader"
	ServicePath       = "/etc/systemd/system/neurader.service"
)

// InstallService configures neurader as a background systemd daemon.
func InstallService() {
	fmt.Println("[*] Setting up neurader as a system service...")

	os.MkdirAll(ConfigDir, 0755)

	self, err := os.Executable()
	if err == nil {
		input, _ := os.ReadFile(self)
		os.WriteFile(BinaryDestination, input, 0755)
	}

	serviceFile := `[Unit]
Description=neurader Security Agent
After=network.target

[Service]
ExecStart=/usr/local/bin/neurader daemon
Restart=always
User=root
WorkingDirectory=/etc/neurader

[Install]
WantedBy=multi-user.target`

	os.WriteFile(ServicePath, []byte(serviceFile), 0644)
	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "neurader").Run()
	exec.Command("systemctl", "start", "neurader").Run()

	fmt.Println("[+] neurader service installed and started.")
}

// CreateneuraderUser prepares the Child nodes with a dedicated automation user.
func CreateneuraderUser() {
	fmt.Println("[*] Preparing 'neurader' user...")
	exec.Command("useradd", "-m", "-s", "/bin/bash", "neurader").Run()
	sshPath := "/home/neurader/.ssh"
	os.MkdirAll(sshPath, 0700)
	exec.Command("chown", "-R", "neurader:neurader", "/home/neurader").Run()
}
