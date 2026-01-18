package system

import (
	"fmt"
	"os"
	"os/exec"
)

// InstallService handles the full setup of the binary and the systemd daemon
func InstallService() {
	fmt.Println("[*] Setting up sentinex as a system service...")

	configDir := "/etc/sentinex"
	binaryDestination := "/usr/local/bin/sentinex"
	servicePath := "/etc/systemd/system/sentinex.service"

	// 1. Ensure the configuration directory exists for hosts.yml and keys
	err := os.MkdirAll(configDir, 0755)
	if err != nil {
		fmt.Printf("[!] Error creating config directory: %v\n", err)
		return
	}

	// 2. Get the path of the binary that is currently running
	self, err := os.Executable()
	if err != nil {
		fmt.Printf("[!] Error finding current binary: %v\n", err)
		return
	}

	// 3. Copy the binary to /usr/local/bin
	// This ensures 'sentinex' is always in the system PATH
	input, _ := os.ReadFile(self)
	err = os.WriteFile(binaryDestination, input, 0755)
	if err != nil {
		fmt.Printf("[!] Error: Could not copy binary to %s. Did you run with sudo?\n", binaryDestination)
		return
	}

	// 4. Define the service configuration
	serviceFile := `[Unit]
Description=sentinex Security Agent
After=network.target

[Service]
# Runs the 'daemon' command in the background
ExecStart=/usr/local/bin/sentinex daemon
Restart=always
User=root
WorkingDirectory=/etc/sentinex

[Install]
WantedBy=multi-user.target`

	// 5. Write the systemd unit file
	err = os.WriteFile(servicePath, []byte(serviceFile), 0644)
	if err != nil {
		fmt.Printf("[!] Error writing service file: %v\n", err)
		return
	}

	// 6. Trigger Linux to load and start the new service
	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "sentinex").Run()
	exec.Command("systemctl", "start", "sentinex").Run()

	fmt.Println("[+] sentinex service installed and started successfully.")
}

// CreatesentinexUser prepares the specialized SSH user on Child nodes
func CreatesentinexUser() {
	fmt.Println("[*] Preparing 'sentinex' user...")
	
	// Create user with home directory, no password, and bash shell
	// We use 'useradd' which is standard on most Linux distros
	exec.Command("useradd", "-m", "-s", "/bin/bash", "sentinex").Run()
	
	// Create the .ssh directory with correct permissions for SSH to work
	sshPath := "/home/sentinex/.ssh"
	os.MkdirAll(sshPath, 0700)
	
	// Ensure sentinex owns their home directory so they can read the authorized_keys
	exec.Command("chown", "-R", "sentinex:sentinex", "/home/sentinex").Run()
}
