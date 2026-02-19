package ssh

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"text/tabwriter"
	"time"

	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
)

// Terminal Colors
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
)

type HostEntry struct {
	Name string `yaml:"name"`
	IP   string `yaml:"ip"`
}

type Inventory struct {
	Hosts []HostEntry `yaml:"hosts"`
}

/* =========================
   SSH KEY MANAGEMENT
========================= */

func GenerateMasterKeys() {
	os.MkdirAll("/etc/neurader", 0700)

	key, _ := rsa.GenerateKey(rand.Reader, 2048)

	privFile, _ := os.Create("/etc/neurader/id_rsa")
	defer privFile.Close()

	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	pem.Encode(privFile, privBlock)
	os.Chmod("/etc/neurader/id_rsa", 0600)

	pub, _ := ssh.NewPublicKey(&key.PublicKey)
	pubBytes := ssh.MarshalAuthorizedKey(pub)
	os.WriteFile("/etc/neurader/id_rsa.pub", pubBytes, 0644)

	fmt.Println("[+] Master SSH keys generated at /etc/neurader")
}

/* =========================
   REMOTE EXECUTION
========================= */

func ExecuteRemote(target, command string) {
	targetIP := resolveTarget(target)

	keyBytes, err := os.ReadFile("/etc/neurader/id_rsa")
	if err != nil {
		fmt.Println("[!] SSH private key not found. Run with sudo.")
		return
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		fmt.Printf("[!] Key parse error: %v\n", err)
		return
	}

	config := &ssh.ClientConfig{
		User:            "neurader",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	client, err := ssh.Dial("tcp", targetIP+":22", config)
	if err != nil {
		fmt.Printf("[%s] %sConnection failed%s: %v\n", target, ColorRed, ColorReset, err)
		return
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		fmt.Printf("[%s] Session error: %v\n", target, err)
		return
	}
	defer session.Close()

	stdout, _ := session.StdoutPipe()
	stderr, _ := session.StderrPipe()

	if err := session.Start(command); err != nil {
		fmt.Printf("[%s] Start failed: %v\n", target, err)
		return
	}

	go streamOutput(target, stdout)
	go streamOutput(target, stderr)

	session.Wait()
}

func ExecuteRemoteMulti(targets []string, command string) {
	var wg sync.WaitGroup
	fmt.Printf("[*] Executing on %d host(s)\n\n", len(targets))

	for _, t := range targets {
		target := strings.TrimSpace(t)
		if target == "" {
			continue
		}
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			fmt.Printf("%s[%s]%s\n", ColorYellow, host, ColorReset)
			ExecuteRemote(host, command)
			fmt.Println()
		}(target)
	}
	wg.Wait()
	fmt.Println("[+] Execution finished.")
}

/* =========================
   HELPERS & UTILS
========================= */

func ExecuteRemoteWithInput(targetIP, command string, input []byte) error {
	keyBytes, err := os.ReadFile("/etc/neurader/id_rsa")
	if err != nil {
		return fmt.Errorf("private key not found")
	}

	signer, _ := ssh.ParsePrivateKey(keyBytes)
	config := &ssh.ClientConfig{
		User:            "neurader",
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}

	client, err := ssh.Dial("tcp", targetIP+":22", config)
	if err != nil {
		return err
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	err = session.Start(command)
	if err != nil {
		return err
	}

	stdin.Write(input)
	stdin.Close()

	return session.Wait()
}

func ListHosts() {
    inv := loadInventory()
    if len(inv.Hosts) == 0 {
        fmt.Println("No hosts in inventory.")
        return
    }

    // Using tabwriter for clean column alignment
    w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
    fmt.Fprintln(w, "ALIAS\tIP ADDRESS\tSTATUS")
    fmt.Fprintln(w, "-----\t----------\t------")

    var wg sync.WaitGroup
    statusMap := make(map[string]string)
    var mu sync.Mutex

    // Check all hosts in parallel
    for _, h := range inv.Hosts {
        wg.Add(1)
        go func(ip string) {
            defer wg.Done()
            s := checkStatus(ip)
            mu.Lock()
            statusMap[ip] = s
            mu.Unlock()
        }(h.IP)
    }
    wg.Wait()

    for _, h := range inv.Hosts {
        fmt.Fprintf(w, "%s\t%s\t%s\n", h.Name, h.IP, statusMap[h.IP])
    }
    w.Flush()
}

// Updated checkStatus to verify actual SSH access
func checkStatus(ip string) string {
    keyBytes, err := os.ReadFile("/etc/neurader/id_rsa")
    if err != nil {
        return ColorYellow + "● Key Missing" + ColorReset
    }

    signer, err := ssh.ParsePrivateKey(keyBytes)
    if err != nil {
        return ColorRed + "● Key Error" + ColorReset
    }

    config := &ssh.ClientConfig{
        User:            "neurader",
        Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
        HostKeyCallback: ssh.InsecureIgnoreHostKey(),
        Timeout:         2 * time.Second, // Quick timeout for status checks
    }

    // Attempt actual SSH connection
    client, err := ssh.Dial("tcp", ip+":22", config)
    if err != nil {
        // Distinguish between "Port Closed" and "Permission Denied"
        if strings.Contains(err.Error(), "unable to authenticate") {
            return ColorYellow + "● Not Synced" + ColorReset
        }
        return ColorRed + "● Offline" + ColorReset
    }
    defer client.Close()

    return ColorGreen + "● Ready" + ColorReset
}

func streamOutput(target string, reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		fmt.Printf("[%s] %s\n", target, scanner.Text())
	}
}

func resolveTarget(target string) string {
	inv := loadInventory()
	for _, h := range inv.Hosts {
		if h.Name == target {
			return h.IP
		}
	}
	return target 
}

func loadInventory() Inventory {
	var inv Inventory
	data, err := os.ReadFile("/etc/neurader/hosts.yml")
	if err != nil {
		return inv
	}
	yaml.Unmarshal(data, &inv)
	return inv
}
