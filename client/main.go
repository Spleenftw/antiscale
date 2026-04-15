package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type Node struct {
	ID        uint   `json:"id"`
	Hostname  string `json:"hostname"`
	PublicKey      string `json:"public_key"`
	PrivateIP      string `json:"private_ip"`
	Endpoint       string `json:"endpoint"`
	Status         string `json:"status"`
	ApprovedRoutes string `json:"approved_routes"`
}

func getOrGenerateKey() (wgtypes.Key, error) {
	keyPath := "/app/data/private.key"
	keyStr, err := os.ReadFile(keyPath)
	if err == nil && len(keyStr) > 0 {
		return wgtypes.ParseKey(string(keyStr))
	}
	newKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return wgtypes.Key{}, err
	}
	os.MkdirAll("/app/data", 0755)
	os.WriteFile(keyPath, []byte(newKey.String()), 0600)
	return newKey, nil
}

func setupWireGuardInterface() error {
	link, err := netlink.LinkByName("wg0")
	if err == nil {
		netlink.LinkDel(link) // Guarantee clean slate
	}
	wgLink := &netlink.GenericLink{LinkAttrs: netlink.LinkAttrs{Name: "wg0"}, LinkType: "wireguard"}
	if err := netlink.LinkAdd(wgLink); err != nil {
		return fmt.Errorf("failed to add wg0 interface: %w", err)
	}
	return netlink.LinkSetUp(wgLink)
}

func assignIPAddress(ip string) error {
	link, _ := netlink.LinkByName("wg0")
	addr, err := netlink.ParseAddr(ip + "/24") // Simplified mesh subnet
	if err != nil {
		return err
	}
	// Flush existing IPs
	addrs, _ := netlink.AddrList(link, netlink.FAMILY_V4)
	for _, a := range addrs {
		netlink.AddrDel(link, &a)
	}

	netlink.AddrReplace(link, addr)
	return nil
}

func syncWireGuard(me wgtypes.Key, peers []Node, acceptRoutes bool) error {
	client, err := wgctrl.New()
	if err != nil {
		return err
	}
	defer client.Close()

	wgLink, _ := netlink.LinkByName("wg0")

	var peerConfigs []wgtypes.PeerConfig
	for _, p := range peers {
		pubKey, err := wgtypes.ParseKey(p.PublicKey)
		if err != nil {
			continue // Skip invalid keys
		}

		_, ipNet, _ := net.ParseCIDR(p.PrivateIP + "/32")
		allowedIPs := []net.IPNet{*ipNet}

		if acceptRoutes && p.ApprovedRoutes != "" {
			routes := strings.Split(p.ApprovedRoutes, ",")
			for _, r := range routes {
				routeStr := strings.TrimSpace(r)
				if routeStr == "" { continue }
				_, rCIDR, err := net.ParseCIDR(routeStr)
				if err == nil {
					allowedIPs = append(allowedIPs, *rCIDR)
					if wgLink != nil {
						route := &netlink.Route{
							LinkIndex: wgLink.Attrs().Index,
							Dst:       rCIDR,
							Scope:     netlink.SCOPE_LINK,
						}
						netlink.RouteReplace(route)
					}
				}
			}
		}

		var endpoint *net.UDPAddr
		if p.Endpoint != "" {
			endpoint, _ = net.ResolveUDPAddr("udp", p.Endpoint)
		}

		peerConfigs = append(peerConfigs, wgtypes.PeerConfig{
			PublicKey:         pubKey,
			AllowedIPs:        allowedIPs,
			Endpoint:          endpoint,
			ReplaceAllowedIPs: true,
		})
	}

	listenPort := 51820
	conf := wgtypes.Config{
		PrivateKey:   &me,
		ListenPort:   &listenPort,
		ReplacePeers: true,
		Peers:        peerConfigs,
	}

	return client.ConfigureDevice("wg0", conf)
}

func main() {
	controllerURL := os.Getenv("CONTROLLER_URL")
	if controllerURL == "" && len(os.Args) >= 2 {
		controllerURL = os.Args[1]
	}
	if controllerURL == "" {
		log.Fatalf("Missing CONTROLLER_URL environment variable.")
	}

	hostname := os.Getenv("NODE_NAME")
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	privKey, err := getOrGenerateKey()
	if err != nil {
		log.Fatalf("Failed to manipulate WireGuard keys: %v", err)
	}
	publicKey := privKey.PublicKey().String()

	advertiseRoutes := os.Getenv("ADVERTISE_ROUTES")
	authKey := os.Getenv("AUTH_KEY")
	acceptRoutesStr := os.Getenv("ACCEPT_ROUTES")
	acceptRoutes := acceptRoutesStr == "true" || acceptRoutesStr == "1"

	fmt.Printf("Starting Antiscale Client on %s\n", hostname)
	fmt.Printf("Public Key: %s\n", publicKey)

	if advertiseRoutes != "" {
		fmt.Println("Enabling iptables MASQUERADE for outbound subnet routing...")
		exec.Command("sysctl", "-w", "net.ipv4.ip_forward=1").Run()
		exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "100.64.0.0/24", "-j", "MASQUERADE").Run()
	}

	// Create Kernel wg0 interface initially
	if err := setupWireGuardInterface(); err != nil {
		fmt.Printf("Warning: Failed to setup kernel wg0 interface (requires CAP_NET_ADMIN): %v\n", err)
	}

	// Register with Controller
	registerURL := fmt.Sprintf("%s/api/register", controllerURL)
	payload := map[string]interface{}{
		"hostname":          hostname,
		"public_key":        publicKey,
		"advertised_routes": advertiseRoutes,
		"accept_routes":     acceptRoutes,
		"auth_key":          authKey,
	}
	reqBody, _ := json.Marshal(payload)

	resp, err := http.Post(registerURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Fatal("Failed to connect to controller:", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var myNode Node
	json.Unmarshal(body, &myNode)

	fmt.Printf("Registered successfully. Assigned IP: %s\nStatus: %s\n", myNode.PrivateIP, myNode.Status)

	if myNode.Status != "approved" {
		fmt.Println("Waiting for admin approval. Please check the dashboard.")
	} else if myNode.PrivateIP != "" {
		assignIPAddress(myNode.PrivateIP)
	}

	// Polling loop
	for {
		time.Sleep(5 * time.Second)
		syncURL := fmt.Sprintf("%s/api/sync/%s", controllerURL, publicKey)
		
		syncResp, err := http.Get(syncURL)
		if err != nil {
			fmt.Println("Sync error:", err)
			continue
		}
		
		if syncResp.StatusCode == 403 {
			fmt.Println("Still pending approval...")
			syncResp.Body.Close()
			continue
		}

		if syncResp.StatusCode == 200 {
			// Update IP in case we just got approved
			if myNode.Status == "pending" {
				myNode.Status = "approved"
				assignIPAddress(myNode.PrivateIP)
			}

			var peers []Node
			syncBody, _ := io.ReadAll(syncResp.Body)
			json.Unmarshal(syncBody, &peers)
			
			// Try to sync to the kernel interface using wgctrl
			if err := syncWireGuard(privKey, peers, acceptRoutes); err != nil {
				fmt.Printf("Failed to configure wireguard: %v\n", err)
			} else {
				fmt.Printf("Successfully synchronized %d approved peers to wg0.\n", len(peers))
			}
		}
		syncResp.Body.Close()
	}
}
