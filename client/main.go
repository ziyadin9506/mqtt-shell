package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const Version = "1.0.0"

type ShellClient struct {
	config    *Config
	client    mqtt.Client
	responses chan Response
}

type Command struct {
	Action  string   `json:"action"`
	Command []string `json:"command"`
}

type Response struct {
	Success   bool                   `json:"success"`
	Message   string                 `json:"message"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

func main() {
	fmt.Printf("Secure MQTT Shell v%s\n", Version)
	fmt.Println("================================")

	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate
	if config.ExecKey == "" {
		log.Fatal("EXEC_KEY is required! Set it in environment or .env file")
	}

	// Create client
	client := NewShellClient(config)

	// Connect
	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Disconnect()

	fmt.Println("\nConnected to MQTT broker")
	fmt.Println("Listening for responses...")
	fmt.Println("\nType commands to execute on remote system.")
	fmt.Println("Special commands:")
	fmt.Println("  exit, quit    - Exit the shell")
	fmt.Println("  clear         - Clear screen")
	fmt.Println("  help          - Show help")
	fmt.Println()

	// Start interactive shell
	client.InteractiveShell()
}

func NewShellClient(config *Config) *ShellClient {
	return &ShellClient{
		config:    config,
		responses: make(chan Response, 10),
	}
}

func (c *ShellClient) Connect() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(c.config.BrokerURL)
	opts.SetClientID(c.config.ClientID)
	opts.SetUsername(c.config.Username)
	opts.SetPassword(c.config.Password)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetOnConnectHandler(c.onConnect)
	opts.SetConnectionLostHandler(c.onConnectionLost)

	if c.config.UseTLS {
		tlsConfig, err := c.config.GetTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to setup TLS: %v", err)
		}
		opts.SetTLSConfig(tlsConfig)
	}

	c.client = mqtt.NewClient(opts)
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (c *ShellClient) Disconnect() {
	if c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(250)
	}
}

func (c *ShellClient) onConnect(client mqtt.Client) {
	// Subscribe to encrypted responses
	topic := c.config.TopicPrefix + "/response/x9vkff7p4"
	if token := client.Subscribe(topic, 1, c.messageHandler); token.Wait() && token.Error() != nil {
		log.Printf("Failed to subscribe: %v", token.Error())
	}
}

func (c *ShellClient) onConnectionLost(client mqtt.Client, err error) {
	fmt.Printf("\nConnection lost: %v\n", err)
	fmt.Println("Will attempt to reconnect...")
}

func (c *ShellClient) messageHandler(client mqtt.Client, msg mqtt.Message) {
	// Decrypt response
	decrypted, err := Decrypt(msg.Payload(), c.config.ExecKey)
	if err != nil {
		log.Printf("Failed to decrypt response: %v", err)
		return
	}

	var resp Response
	if err := json.Unmarshal(decrypted, &resp); err != nil {
		log.Printf("Failed to parse response: %v", err)
		return
	}

	c.responses <- resp
}

func (c *ShellClient) SendCommand(cmdParts []string) error {
	cmd := Command{
		Action:  "exec",
		Command: cmdParts,
	}

	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %v", err)
	}

	// Encrypt
	encrypted, err := Encrypt(payload, c.config.ExecKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt command: %v", err)
	}

	// Publish
	topic := c.config.TopicPrefix + "/x9vkff7p4"
	token := c.client.Publish(topic, 1, false, encrypted)
	token.Wait()

	return token.Error()
}

func (c *ShellClient) InteractiveShell() {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\nremote> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("Error reading input: %v", err)
			continue
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Handle special commands
		switch input {
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return
		case "clear":
			fmt.Print("\033[H\033[2J")
			continue
		case "help":
			c.showHelp()
			continue
		}

		// Parse command
		parts := parseCommand(input)
		if len(parts) == 0 {
			continue
		}

		// Send command
		if err := c.SendCommand(parts); err != nil {
			fmt.Printf("Error sending command: %v\n", err)
			continue
		}

		fmt.Println("Waiting for response...")

		// Wait for response with timeout
		select {
		case resp := <-c.responses:
			c.displayResponse(resp)
		case <-time.After(15 * time.Second):
			fmt.Println("Response timeout - server may be offline")
		}
	}
}

func (c *ShellClient) displayResponse(resp Response) {
	if resp.Success {
		fmt.Println("\nSuccess:", resp.Message)
	} else {
		fmt.Println("\nFailed:", resp.Message)
	}

	if resp.Data != nil {
		if cmd, ok := resp.Data["command"].(string); ok {
			fmt.Printf("\nCommand: %s\n", cmd)
		}

		if output, ok := resp.Data["output"].(string); ok {
			fmt.Println("\nOutput:")
			fmt.Println("---")
			fmt.Print(output)
			if !strings.HasSuffix(output, "\n") {
				fmt.Println()
			}
			fmt.Println("---")
		}

		if errMsg, ok := resp.Data["error"].(string); ok {
			fmt.Printf("\nError: %s\n", errMsg)
		}
	}

	fmt.Printf("\nTime: %s\n", resp.Timestamp.Format("2006-01-02 15:04:05"))
}

func (c *ShellClient) showHelp() {
	fmt.Println("\nSecure MQTT Shell - Help")
	fmt.Println("===========================")
	fmt.Println("\nCommands are executed on the remote system.")
	fmt.Println("\nExamples:")
	fmt.Println("  docker ps                    - List containers")
	fmt.Println("  docker port webserver        - Show port mappings")
	fmt.Println("  netstat -tulpn | grep 9080   - Check if port is listening")
	fmt.Println("  curl http://localhost:9080   - Test local access")
	fmt.Println("  ip addr show                 - Show network interfaces")
	fmt.Println("  ps aux                       - List processes")
	fmt.Println("  free -m                      - Show memory usage")
	fmt.Println("  df -h                        - Show disk usage")
	fmt.Println("\nShell Special commands:")
	fmt.Println("  exit, quit   - Exit remote shell")
	fmt.Println("  clear        - Clear screen")
	fmt.Println("  help         - Show this help")
}

func parseCommand(input string) []string {
	// Simple command parser that respects quotes
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, char := range input {
		switch {
		case char == '"' || char == '\'':
			if inQuote {
				if char == quoteChar {
					inQuote = false
					quoteChar = 0
				} else {
					current.WriteRune(char)
				}
			} else {
				inQuote = true
				quoteChar = char
			}
		case char == ' ' && !inQuote:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(char)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
