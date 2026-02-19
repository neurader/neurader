package main

import (
	"fmt"
	"os"
	"strings"
	"path/filepath" // Added for path handling

	"neurader/internal/api"
	"neurader/internal/ssh"
	"neurader/internal/system"
)

const Version = "v2.0.0"

func main() {
	if len(os.Args) < 2 {
		showHelp()
		return
	}

	switch os.Args[1] {
	case "version":
		fmt.Printf("neurader Version: %s\n", Version)

	case "install":
		runWizard()

	case "init":
		fmt.Println("🚀 Initializing Handshake with Inventory...")
		api.ProactiveHandshake()

	case "upgrade":
		fmt.Printf("🚀 neurader %s Global Upgrade\n", Version)
		fmt.Println("---------------------------------")
		err := system.FetchAndUpgradeJumpbox()
		if err != nil {
			fmt.Printf("[!] Jumpbox upgrade failed: %v\n", err)
			return
		}
		ssh.UpdateAllChildren()
		fmt.Println("\n✨ Global upgrade complete. All nodes are on the latest build.")

	case "daemon":
		fmt.Printf("[*] neurader Daemon %s is active...\n", Version)
		api.StartRegistrationServer("9090")

	case "pending":
		api.ListPending()

	case "accept":
		if len(os.Args) < 3 {
			fmt.Println("Error: Provide the IP of the child to accept.")
			return
		}
		api.AcceptHost(os.Args[2])

	case "list":
		ssh.ListHosts()

	case "add":
        if len(os.Args) < 4 {
            fmt.Println("Usage: neurader add <Alias> <IP>")
            return
        }
        alias, ip := os.Args[2], os.Args[3]

        if err := os.MkdirAll(filepath.Dir(api.InventoryPath), 0755); err != nil {
            fmt.Printf("[!] Permission Error: %v\n", err)
            return
        }

        inventory := api.LoadFile(api.InventoryPath)
        if inventory.Hosts == nil {
            inventory.Hosts = []api.HostEntry{}
        }
        
        inventory.Hosts = append(inventory.Hosts, api.HostEntry{Name: alias, IP: ip})
        api.WriteData(api.InventoryPath, inventory)
        
        fmt.Printf("[+] Manually added %s (%s) to inventory.\n", alias, ip)

        // NEW: Automatically attempt to sync the key to the new node
        fmt.Println("[*] Attempting automatic sync...")
        api.ProactiveHandshake()
		
	case "run":
		if len(os.Args) < 4 {
			fmt.Println("Usage: neurader run <Alias/IP> \"command\"")
			return
		}
		targets := strings.Split(os.Args[2], ",")
		ssh.ExecuteRemoteMulti(targets, os.Args[3])

	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		showHelp()
	}
}

func showHelp() {
	fmt.Printf("neurader %s - Automation & Security Tool\n", Version)
	fmt.Println("Usage: neurader [version | upgrade | install | daemon | pending | accept <IP> | list | add <Alias> <IP> | run <Alias/IP> <cmd>]")
}

func runWizard() {
	fmt.Printf("🛡️ neurader %s Setup Wizard\n", Version)
	fmt.Println("---------------------------")
	fmt.Println("1) Jumpbox (Manager Role)")
	fmt.Println("2) Child    (Thin Agent Role)")
	fmt.Print("\nSelect Role [1-2]: ")

	var choice int
	fmt.Scanln(&choice)

	if choice == 1 {
		fmt.Println("[*] Configuring Jumpbox...")
		
		// Ensure the directory exists FIRST
		configDir := "/etc/neurader"
		if err := os.MkdirAll(configDir, 0755); err != nil {
			fmt.Printf("[!] Fatal: Could not create %s. Run with sudo.\n", configDir)
			return
		}

		ssh.GenerateMasterKeys()
		
		// Initialize the file if it's missing
		hostsFile := filepath.Join(configDir, "hosts.yml")
		if _, err := os.Stat(hostsFile); os.IsNotExist(err) {
			os.WriteFile(hostsFile, []byte("hosts: []\n"), 0644)
			fmt.Println("[+] Created initial inventory at", hostsFile)
		}

		system.InstallService()
		fmt.Println("\n[+] Jumpbox Installation Successful!")
		
	} else if choice == 2 {
		fmt.Println("[*] Configuring Thin Agent...")
		system.CreateneuraderUser() 
		system.InstallService()

		fmt.Print("Enter Jumpbox IP: ")
		var jip string
		fmt.Scanln(&jip)
		
		api.SendRequest(jip)
		fmt.Println("\n[+] Child Agent is now signaling the Jumpbox.")
	} else {
		fmt.Println("[!] Invalid selection. Exiting.")
	}
}
