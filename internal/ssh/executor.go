package ssh

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net"
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
	os.MkdirAll("/etc/sentinex", 0700)

	key, _ := rsa.GenerateKey(rand.Reader, 2048)

	privFile, _ := os.Create("/etc/sentinex/id_rsa")
	defer privFile.Close()

	privBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	pem.Encode(privFile, privBlock)
	os.Chmod("/etc/sentinex/id_rsa", 0600)

	pub, _ := ssh.NewPublicKey(&key.PublicKey)
	pubBytes := ssh.MarshalAuthorizedKey(pub)
	os.WriteFile("/etc/sentinex/id_rsa.pub", pubBytes, 0644)

	fmt.Println("[+] Master SSH keys generated at /etc/sentinex")
}

/* =========================
   REMOTE EXECUTION
========================= */

func ExecuteRemote(target, command string) {
	targetIP := resolveTarget(target)

	keyBytes, err := os.ReadFile("/etc/sentinex/id_rsa")
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
		User:            "sentinex",
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
   v2 CASCADING UPGRADE
========================= */

func UpdateAllChildren() {
	inv := loadInventory()
	if len(inv.Hosts) == 0 {
		fmt.Println("No hosts in inventory to update.")
		return
	}

	// Read the already updated binary from the Jumpbox local disk
	binaryData, err := os.ReadFile("/usr/local/bin/sentinex")
	if err != nil {
		fmt.Printf("[!] Error reading Jumpbox binary: %v\n", err)
		return
	}

	fmt.Printf("[*] Starting cascading update for %d hosts...\n", len(inv.Hosts))

	var wg sync.WaitGroup
	for _, h := range inv.Hosts {
		wg.Add(1)
		go func(host HostEntry) {
			defer wg.Done()

			// Update sequence:
			// 1. Stream binary to /tmp
			// 2. Move to /usr/local/bin (requires sudo)
			// 3. Restart the systemd daemon to apply v2
			updateCmd := "cat > /tmp/sentinex.new && sudo mv /tmp/sentinex.new /usr/local/bin/sentinex && sudo chmod +x /usr/local/bin/sentinex && sudo systemctl restart sentinex"

			err := ExecuteRemoteWithInput(host.IP, updateCmd, binaryData)
			if err != nil {
				fmt.Printf("[%s] %sUpdate Failed%s: %v\n", host.Name, ColorRed, ColorReset, err)
			} else {
				fmt.Printf("[%s] %sUpdate Successful%s\n", host.Name, ColorGreen, ColorReset)
			}
		}(h)
	}
	wg.Wait()
	fmt.Println("[+] Cascading update process finished.")
}

/* =========================
   HELPERS & UTILS
========================= */

func ExecuteRemoteWithInput(targetIP, command string, input []byte) error {
	keyBytes, err := os.ReadFile("/etc/sentinex/id_rsa")
	if err != nil {
		return fmt.Errorf("private key not found")
	}

	signer, _ := ssh.ParsePrivateKey(keyBytes)
	config := &ssh.ClientConfig{
		User:            "sentinex",
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

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ALIAS\tIP ADDRESS\tSTATUS")
	fmt.Fprintln(w, "-----\t----------\t------")

	var wg sync.WaitGroup
	status := make(map[string]string)
	var mu sync.Mutex

	for _, h := range inv.Hosts {
		wg.Add(1)
		go func(ip string) {
			defer wg.Done()
			s := checkStatus(ip)
			mu.Lock()
			status[ip] = s
			mu.Unlock()
		}(h.IP)
	}
	wg.Wait()

	for _, h := range inv.Hosts {
		fmt.Fprintf(w, "%s\t%s\t%s\n", h.Name, h.IP, status[h.IP])
	}
	w.Flush()
}

func checkStatus(ip string) string {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, "22"), 2*time.Second)
	if err != nil {
		return ColorRed + "● Disconnected" + ColorReset
	}
	conn.Close()
	return ColorGreen + "● Connected" + ColorReset
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
	data, err := os.ReadFile("/etc/sentinex/hosts.yml")
	if err != nil {
		return inv
	}
	yaml.Unmarshal(data, &inv)
	return inv
}
