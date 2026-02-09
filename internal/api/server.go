package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"gopkg.in/yaml.v3"
)

// Standardizing paths for v2
const (
	ConfigDir     = "/etc/neurader"
	InventoryPath = "/etc/neurader/hosts.yml"
	PendingPath   = "/etc/neurader/pending_hosts.yml"
	MasterPubKey  = "/etc/neurader/id_rsa.pub"
)

type HostEntry struct {
	Name 	string `yaml:"name"`
	IP   	string `yaml:"ip"`
	Synced 	bool   `yaml:"synced"`
}

type Inventory struct {
	Hosts []HostEntry `yaml:"hosts"`
}

/* =========================
   JUMPBOX REGISTRATION
========================= */

// StartRegistrationServer runs on the Jumpbox to listen for new child nodes
func StartRegistrationServer(port string) {
	// Clean up old requests on start to keep inventory fresh
	writeData(PendingPath, Inventory{Hosts: []HostEntry{}})
	
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		hostname := r.URL.Query().Get("host")
		ip := strings.Split(r.RemoteAddr, ":")[0]

		savePending(hostname, ip)

		fmt.Printf("\n[!] New Registration Request: %s (%s)", hostname, ip)
		fmt.Printf("\nAction required: sudo neurader accept %s\n> ", ip)
	})

	fmt.Printf("[*] neurader Registration Service (v2) listening on port %s...\n", port)
	http.ListenAndServe(":"+port, nil)
}

func savePending(name, ip string) {
	inv := loadFile(PendingPath)
	for _, h := range inv.Hosts {
		if h.IP == ip { return }
	}
	inv.Hosts = append(inv.Hosts, HostEntry{Name: name, IP: ip})
	writeData(PendingPath, inv)
}

func AcceptHost(childIP string) {
	pending := loadFile(PendingPath)
	inventory := loadFile(InventoryPath)
	
	var targetEntry HostEntry
	var newPending []HostEntry
	found := false

	for _, h := range pending.Hosts {
		if h.IP == childIP {
			targetEntry = h
			found = true
		} else {
			newPending = append(newPending, h)
		}
	}

	if !found {
		fmt.Printf("[!] Error: IP %s is not currently requesting registration.\n", childIP)
		return
	}

	fmt.Printf("[?] Enter custom alias for %s (Default: %s): ", childIP, targetEntry.Name)
	var alias string
	fmt.Scanln(&alias)
	if alias == "" {
		alias = targetEntry.Name
	}

	// Read the Jumpbox Public Key to send to the child
	pubKey, err := os.ReadFile(MasterPubKey)
	if err != nil {
		fmt.Printf("[!] Critical Error: Public key not found at %s\n", MasterPubKey)
		return
	}

	// Finalize handshake with the child
	url := fmt.Sprintf("http://%s:9091/finalize", childIP)
	resp, err := http.Post(url, "text/plain", bytes.NewBuffer(pubKey))
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("[!] Handshake failed with %s. Is the child agent running?\n", childIP)
		return
	}

	inventory.Hosts = append(inventory.Hosts, HostEntry{Name: alias, IP: childIP})
	writeData(InventoryPath, inventory)
	writeData(PendingPath, Inventory{Hosts: newPending})

	fmt.Printf("[+] Success! %s (%s) is now in the active inventory.\n", alias, childIP)
}

/* =========================
   CHILD SIDE LOGIC
========================= */

// SendRequest runs on the Child Node to signal the Jumpbox
func SendRequest(jumpboxIP string) {
	hostname, _ := os.Hostname()
	url := fmt.Sprintf("http://%s:9090/register?host=%s", jumpboxIP, hostname)
	
	fmt.Printf("[*] Sending registration request to Jumpbox (%s)...\n", jumpboxIP)
	_, err := http.Post(url, "text/plain", nil)
	if err != nil {
		fmt.Printf("[!] Could not connect to Jumpbox: %v\n", err)
		return
	}

	// Temporary server to receive the Master SSH Key
	mux := http.NewServeMux()
	server := &http.Server{Addr: ":9091", Handler: mux}

	mux.HandleFunc("/finalize", func(w http.ResponseWriter, r *http.Request) {
		key, _ := io.ReadAll(r.Body)
		
		// 1. Setup SSH Key for the 'neurader' user
		os.MkdirAll("/home/neurader/.ssh", 0700)
		os.WriteFile("/home/neurader/.ssh/authorized_keys", key, 0600)
		exec.Command("chown", "-R", "neurader:neurader", "/home/neurader/.ssh").Run()

		// 2. Setup Passwordless Sudo for automation
		sudoRule := "neurader ALL=(ALL) NOPASSWD:ALL\n"
		sudoPath := "/etc/sudoers.d/neurader"
		_ = os.WriteFile(sudoPath, []byte(sudoRule), 0440)

		fmt.Println("\n[+] Success! Master key received and sudo permissions granted.")
		w.WriteHeader(http.StatusOK)
		
		// Shutdown the handshake server after success
		go func() { server.Close() }()
	})
	
	fmt.Println("[*] Awaiting administrator approval on Jumpbox...")
	server.ListenAndServe()
}

/* =========================
   HELPERS
========================= */

func ListPending() {
	inv := loadFile(PendingPath)
	if len(inv.Hosts) == 0 {
		fmt.Println("No active registration requests.")
		return
	}
	fmt.Println("Current Pending Requests:")
	for _, h := range inv.Hosts {
		fmt.Printf(" - %s (%s)\n", h.Name, h.IP)
	}
}

func loadFile(path string) Inventory {
	var inv Inventory
	data, err := os.ReadFile(path)
	if err != nil { return inv }
	_ = yaml.Unmarshal(data, &inv)
	return inv
}

func writeData(path string, inv Inventory) {
	data, _ := yaml.Marshal(inv)
	_ = os.MkdirAll(ConfigDir, 0755)
	err := os.WriteFile(path, data, 0644)
	if err != nil {
		fmt.Printf("[!] PERMISSION ERROR: Could not write to %s. Did you use sudo?\n", path)
	}

// ProactiveHandshake iterates through manual hosts.yml entries and completes the handshake
func ProactiveHandshake() {
	inventory := loadFile(InventoryPath)
	if len(inventory.Hosts) == 0 {
		fmt.Println("[!] Inventory is empty. Please add nodes to /etc/neurader/hosts.yml first.")
		return
	}

	pubKey, err := os.ReadFile(MasterPubKey)
	if err != nil {
		fmt.Printf("[!] Critical Error: Public key not found at %s. Did you run 'install' first?\n", MasterPubKey)
		return
	}

	fmt.Printf("[*] Attempting handshake with %d nodes...\n", len(inventory.Hosts))

	for _, host := range inventory.Hosts {
		// Use the finalize endpoint on the child (Port 9091)
		url := fmt.Sprintf("http://%s:9091/finalize", host.IP)
		
		fmt.Printf(" -> Connecting to %s (%s)... ", host.Name, host.IP)
		
		resp, err := http.Post(url, "text/plain", bytes.NewBuffer(pubKey))
		if err != nil {
			fmt.Printf("FAILED: %v\n", err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			fmt.Println("SUCCESS ✅")
		} else {
			fmt.Printf("FAILED (Status: %d)\n", resp.StatusCode)
		}
		resp.Body.Close()
		}
	}
}
