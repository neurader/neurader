package ssh

import (
	"fmt"
	"os"
	"sync"
)

/* ========================================================================
   v2 CASCADING UPGRADE ENGINE
   This file handles the distribution of the neurader binary from the 
   Jumpbox to all managed child nodes defined in hosts.yml.
   ======================================================================== */

// UpdateAllChildren reads the locally updated binary on the Jumpbox 
// and pushes it to all hosts in parallel via SSH.


	fmt.Printf(ColorGreen+"[*] neurader v2: Blasting update to %d child nodes...\n"+ColorReset, len(inv.Hosts))

	var wg sync.WaitGroup
	for _, h := range inv.Hosts {
		wg.Add(1)
		go func(host HostEntry) {
			defer wg.Done()
			
			// THE UPGRADE SEQUENCE (Remote Execution):
			// 1. Receive binary stream and save to /tmp/neurader.new
			// 2. Move to /usr/local/bin (Atomic swap using sudo)
			// 3. Set executable permissions
			// 4. Restart the systemd service to load the new v2 code
			updateCmd := "cat > /tmp/neurader.new && sudo mv /tmp/neurader.new /usr/local/bin/neurader && sudo chmod +x /usr/local/bin/neurader && sudo systemctl restart neurader"
			
			// ExecuteRemoteWithInput (from executor.go) handles the SSH tunnel and stdin stream
			err := ExecuteRemoteWithInput(host.IP, updateCmd, binaryData)
			if err != nil {
				fmt.Printf("[%s] %sUpdate Failed%s: %v\n", host.Name, ColorRed, ColorReset, err)
			} else {
				fmt.Printf("[%s] %sSuccessfully Patched & Restarted%s\n", host.Name, ColorGreen, ColorReset)
			}
		}(h)
	}

	// Wait for all concurrent updates to finish
	wg.Wait()
	fmt.Println("\n" + ColorGreen + "[+++] Global update complete. All nodes are synchronized." + ColorReset)
}
