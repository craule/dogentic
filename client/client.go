package client

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/Dogentadmin/dogent-agent/executor"
	"github.com/Dogentadmin/dogent-agent/updater"
	"github.com/gorilla/websocket"
)

type Config struct {
	ServerURL string
	Token     string
	ServerID  string
	Version   string
	PublicKey string // Base64 Ed25519 Public Key
}

type AgentClient struct {
	config   Config
	conn     *websocket.Conn
	done     chan struct{}
	mu       sync.Mutex
	verifier ed25519.PublicKey
}

func NewAgentClient(cfg Config) *AgentClient {
	// Parse Public Key once
	pubBytes, err := base64.StdEncoding.DecodeString(cfg.PublicKey)
	if err != nil {
		log.Printf("⚠️ Warning: Invalid Public Key provided. Signature verification will fail. Error: %v", err)
	}

	return &AgentClient{
		config:   cfg,
		done:     make(chan struct{}),
		verifier: ed25519.PublicKey(pubBytes),
	}
}

func (c *AgentClient) Connect() {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u, err := url.Parse(c.config.ServerURL)
	if err != nil {
		log.Fatal("Invalid URL:", err)
	}

	retryDelay := 1 * time.Second
	maxDelay := 60 * time.Second

	for {
		log.Printf("🔌 Connecting to %s...", u.String())

		// Connect
		conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
		if err != nil {
			log.Printf("❌ Connection failed: %v. Retrying in %v...", err, retryDelay)
			time.Sleep(retryDelay)

			// Exponential Backoff
			retryDelay *= 2
			if retryDelay > maxDelay {
				retryDelay = maxDelay
			}
			continue
		}

		// Reset delay on success
		retryDelay = 1 * time.Second

		c.conn = conn
		log.Println("✅ Connected!")

		// Authenticate
		authMsg := map[string]string{
			"token":     c.config.Token,
			"server_id": c.config.ServerID,
			"version":   c.config.Version,
		}

		if err := c.SendMessage(authMsg); err != nil {
			log.Println("Authentication write failed:", err)
			c.conn.Close()
			continue
		}

		// Listen Loop
		c.listen()

		// If listen returns, it means we disconnected. Loop continues to reconnect.
	}
}

func (c *AgentClient) listen() {
	defer c.conn.Close()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			return
		}

		// Handle Message
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("JSON Parse Error: %v", err)
			continue
		}

		c.handleMessage(msg)
	}
}

func (c *AgentClient) handleMessage(msg map[string]interface{}) {
	msgType, ok := msg["type"].(string)
	if !ok {
		return
	}

	switch msgType {
	case "status":
		log.Printf("ℹ️ Status: %v", msg["content"])
	case "pong":
		// Heartbeat ack, ignore
	case "upgrade":
		log.Println("⚡ Upgrade command received. Starting update process...")
		c.SendMessage(map[string]string{"type": "status", "content": "Upgrading agent..."})

		if err := updater.UpdateAgent(c.config.Version); err != nil {
			log.Printf("❌ Update failed: %v", err)
			c.SendMessage(map[string]string{"type": "error", "content": fmt.Sprintf("Update failed: %v", err)})
		}

	case "command":
		cmdContent, _ := msg["content"].(string)
		
		// 1. Extract Metadata
		timestampFloat, _ := msg["timestamp"].(float64)
		timestamp := int64(timestampFloat)
		signature, _ := msg["signature"].(string)

		log.Printf("📥 Received Command: %s (TS: %d)", cmdContent, timestamp)

		// 2. Security Checks
		if c.verifier == nil || len(c.verifier) != ed25519.PublicKeySize {
			log.Println("🛑 Security Error: No valid Public Key configured. Rejecting command.")
			return
		}

		// Check Freshness (prevent replay attacks, allow 60s window)
		now := time.Now().Unix()
		if now-timestamp > 60 || timestamp > now+10 { // +10 for clock skew
			log.Printf("🛑 Security Error: Command expired or future timestamp. (Delay: %ds)", now-timestamp)
			return
		}

		// Verify Signature
		// Match Python's signing format: "{timestamp}:{content}"
		payload := fmt.Sprintf("%d:%s", timestamp, cmdContent)
		sigBytes, err := base64.StdEncoding.DecodeString(signature)
		if err != nil {
			log.Println("🛑 Security Error: Invalid signature format.")
			return
		}

		if !ed25519.Verify(c.verifier, []byte(payload), sigBytes) {
			log.Println("🛑 Security Error: INVALID SIGNATURE! Command might be tampered.")
			return
		}

		log.Println("✅ Signature Verified. Executing command...")

		// Execute in Goroutine to avoid blocking the read loop
		go func(cmd string) {
			output, err := executor.RunCommand(cmd)
			if err != nil {
				log.Printf("Execution Error: %v", err)
				output = fmt.Sprintf("Error: %v", err)
			}

			// Send Result
			result := map[string]string{
				"type":    "command_result",
				"content": output,
			}
			c.SendMessage(result)
		}(cmdContent)

	default:
		log.Printf("Unknown message: %v", msg)
	}
}

// SendMessage sends a JSON message to the websocket connection.
// It is thread-safe.
func (c *AgentClient) SendMessage(payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("connection not established")
	}
	return c.conn.WriteJSON(payload)
}
