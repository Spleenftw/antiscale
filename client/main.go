package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

type Node struct {
	ID        uint   `json:"id"`
	Hostname  string `json:"hostname"`
	PublicKey string `json:"public_key"`
	PrivateIP string `json:"private_ip"`
	Status    string `json:"status"`
}

func main() {
	controllerURL := os.Getenv("CONTROLLER_URL")
	if controllerURL == "" && len(os.Args) >= 2 {
		controllerURL = os.Args[1]
	}
	if controllerURL == "" {
		log.Fatalf("Missing CONTROLLER_URL environment variable or argument.\nExample: CONTROLLER_URL=http://localhost:8080 %s", os.Args[0])
	}

	hostname := os.Getenv("NODE_NAME")
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	
	// Mock wireguard key generation for MVP
	// In production, this would use wgctrl or exec "wg genkey"
	mockPublicKey := os.Getenv("NODE_KEY")
	if mockPublicKey == "" {
		mockPublicKey = fmt.Sprintf("PubKey_%s_%d", hostname, time.Now().Unix())
	}

	advertiseRoutes := os.Getenv("ADVERTISE_ROUTES")
	
	// New Advanced Flags mimicking tailscale commands
	authKey := os.Getenv("AUTH_KEY")
	acceptRoutesStr := os.Getenv("ACCEPT_ROUTES")
	acceptRoutes := acceptRoutesStr == "true" || acceptRoutesStr == "1"
	
	fmt.Printf("Starting Antiscale Client on %s\n", hostname)
	fmt.Printf("Generated Mock Public Key: %s\n", mockPublicKey)
	if advertiseRoutes != "" {
		fmt.Printf("Advertising Routes: %s\n", advertiseRoutes)
	}
	if acceptRoutes {
		fmt.Printf("Accepting Routes: %v\n", acceptRoutes)
	}
	if authKey != "" {
		fmt.Println("Attempting SSO Pre-Auth with provided Auth Key...")
	}
	
	// Register with Controller
	registerURL := fmt.Sprintf("%s/api/register", controllerURL)
	payload := map[string]interface{}{
		"hostname":          hostname,
		"public_key":        mockPublicKey,
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
	}

	// Polling loop
	for {
		time.Sleep(10 * time.Second)
		syncURL := fmt.Sprintf("%s/api/sync/%s", controllerURL, mockPublicKey)
		
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
			var peers []Node
			syncBody, _ := io.ReadAll(syncResp.Body)
			json.Unmarshal(syncBody, &peers)
			
			fmt.Printf("Synced %d approved peers.\n", len(peers))
			for _, p := range peers {
				fmt.Printf(" -> Peer %s (%s) @ %s\n", p.Hostname, p.PrivateIP, p.PublicKey)
			}
			
			// Here we would sync with WireGuard OS Interface
			// writeWgConfig(me, peers)
		}
		syncResp.Body.Close()
	}
}
