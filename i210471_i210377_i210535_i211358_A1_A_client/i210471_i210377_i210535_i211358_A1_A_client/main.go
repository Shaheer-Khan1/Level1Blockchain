package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "io/ioutil"
    "math/rand"
    "net"
    "net/http"
    "os"
    "os/exec"
    "strconv"
    "path/filepath"
    "sync"
    "time"

    shell "github.com/ipfs/go-ipfs-api"
)

type Transaction struct {
    Sender   string            `json:"sender"`
    Receiver string            `json:"receiver"`
    Metadata map[string]string `json:"metadata"`
}

type Block struct {
    BlockID      int           `json:"block_id"`
    Timestamp    time.Time     `json:"timestamp"`
    DatasetID    string        `json:"dataset_id"`
    DatasetAns   string        `json:"dataset_ans"`
    Transactions []Transaction `json:"transactions"`
    PrevHash     string        `json:"prev_hash"`
    Nonce        int          `json:"nonce"`
    Hash         string        `json:"hash"`
}

type Ledger struct {
	Blocks []Block `json:"blocks"`
}

type Message struct {
    Type    string      `json:"type"`
    Payload interface{} `json:"payload"`
    From    string      `json:"from"`
}


var (
    ValidatedTransactions []Transaction
    BlockLedger          struct {
        Blocks []Block `json:"blocks"`
    }
    mu                   sync.RWMutex
    ipfs                 *shell.Shell
    isMining            bool = true
    localAddr           string    
    peerAddr            string    
)

// Initialize peer addresses based on local address
func initPeerAddresses(myPort string) {
    tcpPortInt, _ := strconv.Atoi(myPort)
    tcpPort := strconv.Itoa(tcpPortInt + 1)  // TCP port will be HTTP port + 1
    
    localAddr = fmt.Sprintf(":%s", tcpPort)
    
    // Set peer address based on local port
    if myPort == "5004" {
        peerAddr = "192.168.188.137:5005" // If we're VM2, peer is VM3's TCP port
    } else {
        peerAddr = "192.168.188.136:5005" // If we're VM3, peer is VM2's TCP port
    }
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func initIPFSDesktop() error {
	ipfs = shell.NewShell("localhost:5001")

	if !ipfs.IsUp() {
		return fmt.Errorf("IPFS Desktop is not running. Please start the AppImage first")
	}

	fmt.Println("Successfully connected to IPFS Desktop")
	return nil
}

func fetchAndRunAlgorithm(algoHash string, datasetPath string) (string, error) {
	tempDir := "./algorithms"
	err := os.MkdirAll(tempDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create algorithms directory: %v", err)
	}

	algoPath := filepath.Join(tempDir, "algorithm")

	// Fetch algorithm from IPFS
	reader, err := ipfs.Cat(algoHash)
	if err != nil {
		return "", fmt.Errorf("failed to fetch algorithm from IPFS: %v", err)
	}
	defer reader.Close()

	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read algorithm content: %v", err)
	}

	sourceFile := algoPath + ".go"
	err = ioutil.WriteFile(sourceFile, content, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write algorithm file: %v", err)
	}

	// Compile algorithm
	cmd := exec.Command("go", "build", "-o", algoPath, sourceFile)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to compile algorithm: %v", err)
	}

	// Run algorithm
	cmd = exec.Command(algoPath, datasetPath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to execute algorithm: %v", err)
	}

	// Calculate output hash
	hasher := sha256.New()
	hasher.Write(output)
	outputHash := hex.EncodeToString(hasher.Sum(nil))

	return outputHash, nil
}

func fetchDataset(hash string) (string, error) {
	downloadDir := "./datasets"
	err := os.MkdirAll(downloadDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	outputPath := filepath.Join(downloadDir, hash+".csv")

	file, err := os.Create(outputPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	reader, err := ipfs.Cat(hash)
	if err != nil {
		return "", fmt.Errorf("failed to fetch from IPFS: %v", err)
	}
	defer reader.Close()

	_, err = io.Copy(file, reader)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %v", err)
	}

	return outputPath, nil
}

func validateTransaction(tx Transaction) error {
	// Extract metadata
	datasetHash := tx.Metadata["dataset_hash"]
	algoHash := tx.Metadata["algorithm_hash"]
	receivedOutputHash := tx.Metadata["output_hash"]

	fmt.Printf("Validating transaction with dataset: %s, algorithm: %s\n", datasetHash, algoHash)

	// Fetch dataset
	datasetPath, err := fetchDataset(datasetHash)
	if err != nil {
		return fmt.Errorf("failed to fetch dataset: %v", err)
	}

	// Run algorithm and get output hash
	outputHash, err := fetchAndRunAlgorithm(algoHash, datasetPath)
	if err != nil {
		return fmt.Errorf("failed to run algorithm: %v", err)
	}

	// Compare hashes
	if outputHash != receivedOutputHash {
		return fmt.Errorf("output hash mismatch. Expected: %s, Got: %s", receivedOutputHash, outputHash)
	}

	return nil
}

func printBlock(block Block) {
	fmt.Println("\n=== New Block Created ===")
	fmt.Printf("Block ID: %d\n", block.BlockID)
	fmt.Printf("Timestamp: %s\n", block.Timestamp.Format(time.RFC3339))
	fmt.Printf("Dataset ID: %s\n", block.DatasetID)
	fmt.Printf("Dataset Answer: %s\n", block.DatasetAns)
	fmt.Printf("Previous Hash: %s\n", block.PrevHash)
	fmt.Printf("Current Hash: %s\n", block.Hash)
	fmt.Printf("Nonce: %d\n", block.Nonce)
	fmt.Println("\nTransactions:")
	for i, tx := range block.Transactions {
		fmt.Printf("\n  Transaction %d:\n", i+1)
		fmt.Printf("    Sender: %s\n", tx.Sender)
		fmt.Printf("    Receiver: %s\n", tx.Receiver)
		fmt.Printf("    Metadata:\n")
		for key, value := range tx.Metadata {
			fmt.Printf("      %s: %s\n", key, value)
		}
	}
	fmt.Println("\n=== End of Block ===")
	fmt.Println("----------------------------------------")
}

func saveLedger() error {
	data, err := json.MarshalIndent(BlockLedger, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal ledger: %v", err)
	}
	return ioutil.WriteFile("blockchain_ledger.json", data, 0644)
}

func loadLedger() error {
	data, err := ioutil.ReadFile("blockchain_ledger.json")
	if err != nil {
		if os.IsNotExist(err) {
			BlockLedger = Ledger{Blocks: []Block{}}
			return nil
		}
		return fmt.Errorf("failed to read ledger file: %v", err)
	}

	err = json.Unmarshal(data, &BlockLedger)
	if err != nil {
		return fmt.Errorf("failed to unmarshal ledger: %v", err)
	}
	return nil
}

func listenForBlocks() {
    // Start TCP listener for blocks on the same port as HTTP server
    listener, err := net.Listen("tcp", localAddr)
    if err != nil {
        fmt.Printf("Failed to start block listener: %v\n", err)
        return
    }
    defer listener.Close()

    fmt.Printf("Listening for blocks on %s\n", localAddr)

    for {
        conn, err := listener.Accept()
        if err != nil {
            fmt.Printf("Error accepting connection: %v\n", err)
            continue
        }

        go handleBlockConnection(conn)
    }
}

func handleBlockConnection(conn net.Conn) {
    defer conn.Close()

    decoder := json.NewDecoder(conn)
    var msg Message
    if err := decoder.Decode(&msg); err != nil {
        fmt.Printf("Error decoding message: %v\n", err)
        return
    }

    if msg.Type == "block" {
        blockData, err := json.Marshal(msg.Payload)
        if err != nil {
            fmt.Printf("Error marshaling block data: %v\n", err)
            return
        }

        var receivedBlock Block
        if err := json.Unmarshal(blockData, &receivedBlock); err != nil {
            fmt.Printf("Error unmarshaling block: %v\n", err)
            return
        }

        mu.Lock()
        defer mu.Unlock()

        // Check if we already have this block ID
        for _, existingBlock := range BlockLedger.Blocks {
            if existingBlock.BlockID == receivedBlock.BlockID {
                // If we have a block with same ID, resolve conflict
                if shouldReplaceExistingBlock(existingBlock, receivedBlock) {
                    // Replace existing block
                    for i, block := range BlockLedger.Blocks {
                        if block.BlockID == receivedBlock.BlockID {
                            BlockLedger.Blocks[i] = receivedBlock
                            break
                        }
                    }
                    fmt.Printf("\nReplaced existing block %d with received block (better timestamp/hash)\n", receivedBlock.BlockID)
                }
                return
            }
        }

        // If block is valid, stop mining and clear pending transactions
        if verifyBlock(receivedBlock) {
            isMining = false
            // Only clear transactions that are in the received block
            ValidatedTransactions = removeProcessedTransactions(ValidatedTransactions, receivedBlock.Transactions)
            
            BlockLedger.Blocks = append(BlockLedger.Blocks, receivedBlock)
            fmt.Printf("\nReceived valid block from peer. Stopping mining.\n")
            printBlock(receivedBlock)
            
            saveLedger()
        }
    }
}

func shouldReplaceExistingBlock(existing Block, received Block) bool {
    // First, compare timestamps
    if existing.Timestamp.After(received.Timestamp) {
        return true
    }
    if existing.Timestamp.Before(received.Timestamp) {
        return false
    }
    
    // If timestamps are equal, compare hashes
    return received.Hash < existing.Hash
}

func removeProcessedTransactions(pending []Transaction, processed []Transaction) []Transaction {
    processedMap := make(map[string]bool)
    
    // Create a unique key for each transaction
    for _, tx := range processed {
        key := fmt.Sprintf("%s-%s-%s-%s", 
            tx.Sender, 
            tx.Receiver, 
            tx.Metadata["dataset_hash"],
            tx.Metadata["timestamp"])
        processedMap[key] = true
    }
    
    // Keep only transactions that weren't processed
    var remaining []Transaction
    for _, tx := range pending {
        key := fmt.Sprintf("%s-%s-%s-%s", 
            tx.Sender, 
            tx.Receiver, 
            tx.Metadata["dataset_hash"],
            tx.Metadata["timestamp"])
        if !processedMap[key] {
            remaining = append(remaining, tx)
        }
    }
    
    return remaining
}

func mineBlock(block *Block, difficulty int) {
	prefix := fmt.Sprintf("%0*d", difficulty, 0)
	for {
		block.Nonce = rand.Int()
		hasher := sha256.New()
		data := fmt.Sprintf("%d|%s|%s|%v|%s|%d", 
			block.BlockID, 
			block.DatasetID, 
			block.DatasetAns, 
			block.Transactions, 
			block.PrevHash,
			block.Nonce)
		hasher.Write([]byte(data))
		block.Hash = hex.EncodeToString(hasher.Sum(nil))
		if block.Hash[:difficulty] == prefix {
			break
		}
	}
}

func verifyBlock(block Block) bool {
    // Verify hash
    hasher := sha256.New()
    data := fmt.Sprintf("%d|%s|%s|%v|%s|%d",
        block.BlockID,
        block.DatasetID,
        block.DatasetAns,
        block.Transactions,
        block.PrevHash,
        block.Nonce)
    hasher.Write([]byte(data))
    calculatedHash := hex.EncodeToString(hasher.Sum(nil))

    if calculatedHash != block.Hash {
        return false
    }

    // Verify proof of work
    if block.Hash[:4] != "0000" {
        return false
    }

    return true
}

func broadcastBlock(block Block) {
    conn, err := net.Dial("tcp", peerAddr)
    if err != nil {
        fmt.Printf("Failed to connect to peer %s: %v\n", peerAddr, err)
        return
    }
    defer conn.Close()

    msg := Message{
        Type:    "block",
        Payload: block,
        From:    localAddr,
    }

    encoder := json.NewEncoder(conn)
    if err := encoder.Encode(msg); err != nil {
        fmt.Printf("Failed to broadcast block: %v\n", err)
        return
    }

    fmt.Printf("Successfully broadcast block to peer %s\n", peerAddr)
}

func createBlock(transactions []Transaction, blockID int, difficulty int) Block {
    datasetID := fmt.Sprintf("dataset_%d", rand.Intn(1000))
    datasetAns := fmt.Sprintf("answer_%d", rand.Intn(1000))

    prevHash := ""
    if len(BlockLedger.Blocks) > 0 {
        prevHash = BlockLedger.Blocks[len(BlockLedger.Blocks)-1].Hash
    }

    block := Block{
        BlockID:      blockID,
        Timestamp:    time.Now(),
        DatasetID:    datasetID,
        DatasetAns:   datasetAns,
        Transactions: transactions,
        PrevHash:     prevHash,
    }

    fmt.Printf("Mining block %d...\n", block.BlockID)
    mineBlock(&block, difficulty)
    
    // Broadcast the block to peer
    go broadcastBlock(block)
    
    printBlock(block)
    return block
}

func handleTransaction(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    mu.Lock()
    if !isMining {
        mu.Unlock()
        http.Error(w, "No longer accepting transactions - block received from peer", http.StatusBadRequest)
        return
    }

    var tx Transaction
    if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
        mu.Unlock()
        http.Error(w, fmt.Sprintf("Failed to decode transaction: %v", err), http.StatusBadRequest)
        return
    }

    fmt.Printf("\nReceived transaction for validation: %+v\n", tx)

    // Validate transaction
    err := validateTransaction(tx)
    if err != nil {
        mu.Unlock()
        fmt.Printf("Transaction validation failed: %v\n", err)
        http.Error(w, fmt.Sprintf("Transaction validation failed: %v", err), http.StatusBadRequest)
        return
    }

    // Store validated transaction and potentially create new block
    ValidatedTransactions = append(ValidatedTransactions, tx)
    if len(ValidatedTransactions) >= 2 {
        block := createBlock(ValidatedTransactions[:2], len(BlockLedger.Blocks)+1, 4)
        BlockLedger.Blocks = append(BlockLedger.Blocks, block)
        ValidatedTransactions = ValidatedTransactions[2:]
        
        if err := saveLedger(); err != nil {
            fmt.Printf("Warning: Failed to save ledger: %v\n", err)
        }
    }
    mu.Unlock()

    fmt.Printf("Transaction validated and stored successfully\n")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "Transaction validated and stored successfully",
    })
}

func getValidatedTransactions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	mu.RLock()
	transactions := ValidatedTransactions
	mu.RUnlock()

	json.NewEncoder(w).Encode(transactions)
}

func getBlocks(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    mu.RLock()
    blocks := BlockLedger.Blocks  // Changed from 'Blocks' to 'BlockLedger.Blocks'
    mu.RUnlock()

    json.NewEncoder(w).Encode(blocks)
}

func main() {
    port := "5004" // Default port
    if len(os.Args) > 1 {
        port = os.Args[1]
    }

    // Initialize peer addresses
    initPeerAddresses(port)

    // Initialize random seed
    rand.Seed(time.Now().UnixNano())

    // Load existing ledger
    if err := loadLedger(); err != nil {
        fmt.Printf("Failed to load ledger: %v\n", err)
        return
    }

    // Initialize IPFS
    err := initIPFSDesktop()
    if err != nil {
        fmt.Printf("Failed to connect to IPFS Desktop: %v\n", err)
        fmt.Println("Please make sure IPFS Desktop AppImage is running")
        return
    }

    // Start block listener in a goroutine
    go listenForBlocks()

    // Setup HTTP handlers
    http.HandleFunc("/validateTransaction", handleTransaction)
    http.HandleFunc("/transactions", getValidatedTransactions)
    http.HandleFunc("/blocks", getBlocks)

    tcpPortInt, _ := strconv.Atoi(port)
    fmt.Printf("Validator running HTTP server on port %s...\n", port)
    fmt.Printf("Validator running TCP listener on port %d...\n", tcpPortInt + 1)
    fmt.Printf("Current blockchain length: %d blocks\n", len(BlockLedger.Blocks))
    
    if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
        fmt.Printf("Server failed to start: %v\n", err)
    }
}
