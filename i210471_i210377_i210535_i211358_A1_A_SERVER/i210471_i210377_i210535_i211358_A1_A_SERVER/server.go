package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
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

type Server struct {
	peers     map[string]net.Conn
	peerMutex sync.RWMutex
	seenTxs   map[string]bool
	txMutex   sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		peers:   make(map[string]net.Conn),
		seenTxs: make(map[string]bool),
	}
}

func (s *Server) broadcastTransaction(tx Transaction) {
	msg := Message{
		Type:    "transaction",
		Payload: tx,
		From:    "server",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		fmt.Printf("Failed to marshal message: %v\n", err)
		return
	}

	s.peerMutex.RLock()
	defer s.peerMutex.RUnlock()

	for addr, conn := range s.peers {
		go func(peerConn net.Conn, peerAddr string) {
			_, err := peerConn.Write(append(data, '\n'))
			if err != nil {
				fmt.Printf("Failed to send to %s: %v\n", peerAddr, err)
				// Remove failed connection
				s.peerMutex.Lock()
				delete(s.peers, peerAddr)
				s.peerMutex.Unlock()
				peerConn.Close()
			}
		}(conn, addr)
	}
}

func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	peerAddr := conn.RemoteAddr().String()
	s.peerMutex.Lock()
	s.peers[peerAddr] = conn
	s.peerMutex.Unlock()

	fmt.Printf("New peer connected: %s\n", peerAddr)

	defer func() {
		s.peerMutex.Lock()
		delete(s.peers, peerAddr)
		s.peerMutex.Unlock()
		fmt.Printf("Peer disconnected: %s\n", peerAddr)
	}()

	decoder := json.NewDecoder(conn)
	for {
		var msg Message
		if err := decoder.Decode(&msg); err != nil {
			return
		}

		// Process received message from peer
		fmt.Printf("Received message from %s: %+v\n", peerAddr, msg)
	}
}



func (s *Server) startSocketServer(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to start socket server: %v", err)
	}
	defer listener.Close()

	fmt.Printf("Socket server listening on %s\n", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("Failed to accept connection: %v\n", err)
			continue
		}
		go s.handleConnection(conn)
	}
}

func main() {
	server := NewServer()

	// Start socket server for peer connections
	go func() {
		err := server.startSocketServer(":5002")
		if err != nil {
			fmt.Printf("Socket server error: %v\n", err)
		}
	}()

	// HTTP endpoint for receiving transactions from main.go
	http.HandleFunc("/receiveTransaction", func(w http.ResponseWriter, r *http.Request) {
		var transaction Transaction
		err := json.NewDecoder(r.Body).Decode(&transaction)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode transaction: %v", err), http.StatusBadRequest)
			return
		}

		fmt.Printf("Received transaction via HTTP: %+v\n", transaction)

		// Broadcast the transaction to all connected peers
		server.broadcastTransaction(transaction)

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Transaction received and broadcast successfully")
	})

	fmt.Println("HTTP server running on port 5001...")
	http.ListenAndServe(":5001", nil)
}

