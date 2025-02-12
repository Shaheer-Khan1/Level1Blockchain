package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

type Transaction struct {
	Sender   string            `json:"sender"`
	Receiver string            `json:"receiver"`
	Metadata map[string]string `json:"metadata"`
}

type Message struct {
	Type    string      `json:"type"`
	Payload Transaction `json:"payload"`
	From    string      `json:"from"`
}

func sendTransactionToValidator(tx Transaction) error {
	data, err := json.Marshal(tx)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %v", err)
	}

	resp, err := http.Post("http://localhost:5004/validateTransaction", "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to send transaction to validator: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := json.Marshal(resp.Body)
		return fmt.Errorf("validator returned error: %s", string(body))
	}

	return nil
}

func main() {
	conn, err := net.Dial("tcp", "192.168.188.135:5002")
	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected to server! Waiting for transactions...")
	decoder := json.NewDecoder(conn)

	for {
		var msg Message
		err := decoder.Decode(&msg)
		if err != nil {
			fmt.Println("Error reading from server:", err)
			return
		}

		if msg.Type == "transaction" {
			fmt.Println("\nReceived new transaction:")
			fmt.Printf("From: %s\n", msg.From)
			fmt.Printf("Sender: %s\n", msg.Payload.Sender)
			fmt.Printf("Receiver: %s\n", msg.Payload.Receiver)
			fmt.Println("Metadata:")
			for key, value := range msg.Payload.Metadata {
				fmt.Printf("  %s: %s\n", key, value)
			}
			
			// Send to validator
			err := sendTransactionToValidator(msg.Payload)
			if err != nil {
				fmt.Printf("Error sending to validator: %v\n", err)
			} else {
				fmt.Println("Transaction sent to validator successfully")
			}
			
			fmt.Println("----------------------------------------")
		}
	}
}
