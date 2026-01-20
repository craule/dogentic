package main

import (
	"flag"
	"log"
	"time"

	"github.com/Dogentadmin/dogent-agent/client"
	"github.com/Dogentadmin/dogent-agent/stats"
)

const (
	Version = "1.0.0"
)

func main() {
	// 1. Parse Args
	serverURL := flag.String("server", "wss://backend.dogent.net/api/v1/agent/connect", "Dogent Server URL")
	token := flag.String("token", "", "Authentication Token")
	serverID := flag.String("id", "", "Server ID")
	flag.Parse()

	if *token == "" || *serverID == "" {
		log.Fatal("❌ Missing required flags: --token and --id are required.")
	}

	log.Printf("🚀 Dogent Agent v%s Starting for Server: %s", Version, *serverID)

	// 2. Initialize Client
	cfg := client.Config{
		ServerURL: *serverURL,
		Token:     *token,
		ServerID:  *serverID,
		Version:   Version,
		PublicKey: "wJh+OZuVdyjhHM8hvbUPIMNTjljYiL12K55YE94VYnQ=", // Ed25519 Public Key
	}
	agent := client.NewAgentClient(cfg)

	// 3. Start Stats Loop (Heartbeat)
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s, err := stats.CollectStats()
			if err != nil {
				log.Printf("Stats error: %v", err)
				continue
			}

			// Send Heartbeat
			msg := map[string]interface{}{
				"type":    "heartbeat",
				"content": s,
			}

			if err := agent.SendMessage(msg); err != nil {
				// Log but don't crash, client might be reconnecting
				// log.Printf("Failed to send heartbeat: %v", err)
			}
		}
	}()

	// 4. Blocking Connect Loop
	agent.Connect()
}
