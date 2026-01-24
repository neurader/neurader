package ssh

import (
	"fmt"
	"os"
	"sync"
)

// UpdateAllChildren reads the local updated binary and pushes it to all hosts.
func UpdateAllChildren() {
	inv := loadInventory()
	if len(inv.Hosts) == 0 {
		fmt.Println("No hosts found in inventory to update.")
		return
	}

	// Read the binary we just updated on the Jumpbox
	binaryData, err := os.ReadFile("/usr/local/bin/sentinex")
	if err != nil {
		fmt.Printf("[!] Error reading local binary: %v\n", err)
		return
	}

	var wg sync.WaitGroup
	for _, h := range inv.Hosts {
		wg.Add(1)
		go func(host HostEntry) {
			defer wg.Done()
			
			// COMMAND: 
			// 1. Receive binary via stdin (cat)
			// 2. Move to proper location
			// 3. Restart service to apply changes
			// We do NOT touch /etc/sentinex or keys here.
			updateCmd := "cat > /tmp/sentinex.new && sudo mv /tmp/sentinex.new /usr/local/bin/sentinex && sudo systemctl restart sentinex"
			
			// You'll need an 'ExecuteWithInput' helper (see below)
			err := ExecuteRemoteWithInput(host.IP, updateCmd, binaryData)
			if err != nil {
				fmt.Printf("[%s] Update failed: %v\n", host.Name, err)
			} else {
				fmt.Printf("[+] %s updated and restarted.\n", host.Name)
			}
		}(h)
	}
	wg.Wait()
}
