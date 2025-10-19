package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const Version = "1.0.0"

type ShellServer struct {
	config *Config
	client mqtt.Client
	ctx    context.Context
	cancel context.CancelFunc
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Printf("Secure MQTT Shell Server v%s", Version)

	// Load configuration
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Validate required config
	if config.ExecKey == "" {
		log.Fatal("EXEC_KEY is required! Set it in environment or .env file")
	}

	// Create server
	server := NewShellServer(config)

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Connect to MQTT
	if err := server.Connect(); err != nil {
		log.Fatalf("Failed to connect to MQTT: %v", err)
	}

	log.Println("Server started successfully")
	log.Printf("Listening on: %s/x9vkff7p4", config.TopicPrefix)
	log.Println("Waiting for commands...")

	// Wait for shutdown signal
	<-sigChan
	log.Println("\nShutting down...")
	server.Shutdown()
}

func NewShellServer(config *Config) *ShellServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &ShellServer{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (s *ShellServer) Connect() error {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(s.config.BrokerURL)
	opts.SetClientID(s.config.ClientID)
	opts.SetUsername(s.config.Username)
	opts.SetPassword(s.config.Password)
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetKeepAlive(60 * time.Second)
	opts.SetOnConnectHandler(s.onConnect)
	opts.SetConnectionLostHandler(s.onConnectionLost)

	if s.config.UseTLS {
		tlsConfig, err := s.config.GetTLSConfig()
		if err != nil {
			return fmt.Errorf("failed to setup TLS: %v", err)
		}
		opts.SetTLSConfig(tlsConfig)
	}

	s.client = mqtt.NewClient(opts)
	if token := s.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}

	return nil
}

func (s *ShellServer) onConnect(client mqtt.Client) {
	log.Println("Connected to MQTT broker")

	// Subscribe to encrypted command topic
	topic := s.config.TopicPrefix + "/x9vkff7p4"
	if token := client.Subscribe(topic, 1, s.messageHandler); token.Wait() && token.Error() != nil {
		log.Printf("Failed to subscribe: %v", token.Error())
		return
	}

	log.Printf("Subscribed to: %s", topic)
}

func (s *ShellServer) onConnectionLost(client mqtt.Client, err error) {
	log.Printf("Connection lost: %v", err)
	log.Println("Will attempt to reconnect...")
}

func (s *ShellServer) messageHandler(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Received encrypted command")

	// Decrypt payload
	decrypted, err := Decrypt(msg.Payload(), s.config.ExecKey)
	if err != nil {
		log.Printf("Decryption failed: %v", err)
		s.sendResponse(false, "Decryption failed", nil)
		return
	}

	// Parse command
	var cmd Command
	if err := json.Unmarshal(decrypted, &cmd); err != nil {
		log.Printf("Invalid command format: %v", err)
		s.sendResponse(false, "Invalid command format", nil)
		return
	}

	// Validate command
	if len(cmd.Command) == 0 {
		log.Println("Empty command received")
		s.sendResponse(false, "No command specified", nil)
		return
	}

	// Execute command
	s.executeCommand(cmd.Command)
}

func (s *ShellServer) executeCommand(cmdParts []string) {
	cmdStr := strings.Join(cmdParts, " ")
	log.Printf("Executing: %s", cmdStr)

	// Create command with timeout
	ctx, cancel := context.WithTimeout(s.ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cmdParts[0], cmdParts[1:]...)
	output, err := cmd.CombinedOutput()

	data := map[string]interface{}{
		"command": cmdStr,
		"output":  string(output),
	}

	if err != nil {
		data["error"] = err.Error()
		log.Printf("Command failed: %v", err)
		s.sendResponse(false, "Command execution failed", data)
		return
	}

	log.Printf("Command executed successfully")
	s.sendResponse(true, "Command executed successfully", data)
}

func (s *ShellServer) sendResponse(success bool, message string, data map[string]interface{}) {
	resp := Response{
		Success:   success,
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}

	// Marshal to JSON
	payload, err := json.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	// Encrypt
	encrypted, err := Encrypt(payload, s.config.ExecKey)
	if err != nil {
		log.Printf("Failed to encrypt response: %v", err)
		return
	}

	// Publish
	topic := s.config.TopicPrefix + "/response/x9vkff7p4"
	token := s.client.Publish(topic, 1, false, encrypted)
	token.Wait()

	if token.Error() != nil {
		log.Printf("Failed to publish response: %v", token.Error())
	} else {
		log.Printf("Response sent (success=%v)", success)
	}
}

func (s *ShellServer) Shutdown() {
	s.cancel()
	if s.client != nil && s.client.IsConnected() {
		s.client.Disconnect(250)
	}
	log.Println("Shutdown complete")
}
