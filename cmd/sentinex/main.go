package main

import (
	"fmt"
	"os"
	"sentinex/internal/api"
	"sentinex/internal/ssh"
	"sentinex/internal/system"
	"strings"
)

// Version 2.0.0 - Semantic Versioning
const Version = "v2.0.0"

func main() {
	// If no arguments, show help
	if len(os.Args) < 2 {
		showHelp()
		return
	}

	switch os.Args[1] {
	case "version":
		fmt.Printf("Sentinex Version: %s\n", Version)

	case "install":
		runWizard()

	case "upgrade":
		fmt.Printf("🚀 Sentinex %s Global Upgrade\n", Version)
		fmt.Println("---------------------------------")
		// 1. Jumpbox pulls new binary from your Dev Server
		err := system.FetchAndUpgradeJumpbox()
		if err != nil {
			fmt.Printf("[!] Jumpbox upgrade failed: %v\n", err)
			return
		}
		// 2. Jumpbox pushes that same binary to all children via SSH
		ssh.UpdateAllChildren()
		fmt.Println("\n✨ Global upgrade complete. All nodes are on the latest build.")

	case "daemon":
		fmt.Printf("[*] Sentinex Daemon %s is active...\n", Version)
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

	case "run":
		if len(os.Args) < 4 {
			fmt.Println("Usage: sentinex run <Alias/IP> \"command\"")
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
	fmt.Printf("Sentinex %s - Automation & Security Tool\n", Version)
	fmt.Println("Usage: sentinex [version | upgrade | install | daemon | pending | accept <IP> | list | run <Alias/IP> <cmd>]")
}

func runWizard() {
	fmt.Printf("🛡️ Sentinex %s Setup Wizard\n", Version)
	fmt.Println("---------------------------")
	fmt.Println("1) Jumpbox (Manager Role)")
	fmt.Println("2) Child   (Thin Agent Role)")
	fmt.Print("\nSelect Role [1-2]: ")

	var choice int
	fmt.Scanln(&choice)

	if choice == 1 {
		fmt.Println("[*] Configuring Jumpbox...")
		ssh.GenerateMasterKeys()
		
		if _, err := os.Stat("/etc/sentinex/hosts.yml"); os.IsNotExist(err) {
			os.MkdirAll("/etc/sentinex", 0755)
			os.WriteFile("/etc/sentinex/hosts.yml", []byte("hosts: []\n"), 0644)
		}

		system.InstallService()
		fmt.Println("\n[+] Jumpbox Installation Successful!")
		
	} else if choice == 2 {
		fmt.Println("[*] Configuring Thin Agent...")
		system.CreatesentinexUser() 
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
