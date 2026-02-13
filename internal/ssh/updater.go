package ssh

import (
	"fmt"
	"os"
	"sync"
)

/* ========================================================================
   v2 CASCADING UPGRADE ENGINE
   Handles the parallel distribution of the updated binary.
   ======================================================================== */

// UpdateAllChildren pushes the Jumpbox's current binary to all managed nodes.
func UpdateAllChildren() {
	// 1. Get the list of nodes (defined in executor.go)
	inv := loadInventory()
	if len(inv.Hosts) == 0 {
		fmt.Println(ColorYellow + "[!] No hosts found in inventory." + ColorReset)
		return
	}

	// 2. Read the local binary that was just updated
	binaryPath := "/usr/local/bin/neurader"
	binaryData, err := os.ReadFile(binaryPath)
	if err != nil {
		fmt.Printf(ColorRed+"[!] Error reading local binary: %v\n"+ColorReset, err)
		return
	}

	fmt.Printf(ColorGreen+"[*] neurader v2: Blasting update to %d nodes...\n"+ColorReset, len(inv.Hosts))

	var wg sync.WaitGroup
	for _, h := range inv.Hosts {
		wg.Add(1)
		go func(host HostEntry) {
			defer wg.Done()

			// The remote command sequence
			updateCmd := "cat > /tmp/neurader.new && " +
				"sudo mv /tmp/neurader.new /usr/local/bin/neurader && " +
				"sudo chmod +x /usr/local/bin/neurader && " +
				"sudo systemctl restart neurader"

			// ExecuteRemoteWithInput (defined in executor.go)
			err := ExecuteRemoteWithInput(host.IP, updateCmd, binaryData)
			if err != nil {
				fmt.Printf("[%s] %sUpdate Failed%s: %v\n", host.Name, ColorRed, ColorReset, err)
			} else {
				fmt.Printf("[%s] %sSuccess%s\n", host.Name, ColorGreen, ColorReset)
			}
		}(h)
	}

	wg.Wait()
	fmt.Println("\n" + ColorGreen + "[+++] Global update complete." + ColorReset)
}
