package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	shell "github.com/ipfs/go-ipfs-api"
	"encoding/json"
)

var (
	Mempool []Transaction
	mu      sync.Mutex
	ipfs    *shell.Shell
)

type Transaction struct {
    Sender   string            `json:"sender"`
    Receiver string            `json:"receiver"`
    Metadata map[string]string `json:"metadata"`
}

const (
	ALGORITHM_HASH = "QmP1SRfLVbndNHWJCppfkVrWHvzkQXVrhpz5o8B1h2C1GN" // Replace with your actual DBSCAN algorithm hash
	TEMP_DIR      = "./algorithms"
)

var datasetHashes = []string{
	"QmXDSyMuWipxBTatdD2go78zx297CU75ysrEKvuzvwRVQC", 
	"QmU7ufkHNompEtZK1Mq4g7xMM3XqTXPxc7G76SBHFyWfda",
	"QmUDgLgfDKTp6vhzVYKXjMEwNs1x5prRikKbgR4A6Y3WBw",
}

func initIPFSDesktop() error {
	ipfs = shell.NewShell("localhost:5003")
	
	if !ipfs.IsUp() {
		return fmt.Errorf("IPFS Desktop is not running. Please start the AppImage first")
	}
	
	fmt.Println("Successfully connected to IPFS Desktop")
	return nil
}

func fetchAlgorithmFromIPFS() (string, error) {
	// Create algorithms directory if it doesn't exist
	err := os.MkdirAll(TEMP_DIR, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create algorithms directory: %v", err)
	}

	algoPath := filepath.Join(TEMP_DIR, "dbscan")
	
	// Check if compiled algorithm already exists
	if _, err := os.Stat(algoPath); err == nil {
		fmt.Printf("Using cached algorithm at %s\n", algoPath)
		return algoPath, nil
	}

	// Fetch algorithm source from IPFS
	reader, err := ipfs.Cat(ALGORITHM_HASH)
	if err != nil {
		return "", fmt.Errorf("failed to fetch algorithm from IPFS: %v", err)
	}
	defer reader.Close()

	// Read algorithm content
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read algorithm content: %v", err)
	}

	// Write algorithm to file
	sourceFile := algoPath + ".go"
	err = ioutil.WriteFile(sourceFile, content, 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write algorithm file: %v", err)
	}

	// Compile the algorithm
	cmd := exec.Command("go", "build", "-o", algoPath, sourceFile)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to compile algorithm: %v", err)
	}

	fmt.Printf("Successfully compiled algorithm to %s\n", algoPath)
	return algoPath, nil
}

func fetchDatasetHashes() []string {
	return datasetHashes
}

func pickRandomDataset(hashes []string) string {
	if len(hashes) == 0 {
		return ""
	}
	return hashes[rand.Intn(len(hashes))]
}

func fetchDatasetFromIPFS(hash string) (string, error) {
	downloadDir := "./datasets"
	
	err := os.MkdirAll(downloadDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create directory: %v", err)
	}

	outputPath := filepath.Join(downloadDir, hash+".csv")
	
	if _, err := os.Stat(outputPath); err == nil {
		fmt.Printf("File already exists at %s\n", outputPath)
		return outputPath, nil
	}

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

	fmt.Printf("Successfully downloaded file to %s\n", outputPath)
	return outputPath, nil
}

func runAlgorithm(algoPath string, datasetPath string) (string, error) {
    fmt.Printf("Running algorithm %s on dataset: %s\n", algoPath, datasetPath)
    
    // Execute the algorithm
    cmd := exec.Command(algoPath, datasetPath)
    output, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to execute algorithm: %v", err)
    }

    // Log the output before hashing
    fmt.Println("Algorithm output:", string(output))  // Add this line to log the output

    // Calculate hash of the output
    hasher := sha256.New()
    hasher.Write(output)
    outputHash := hex.EncodeToString(hasher.Sum(nil))

    // Log the output hash
    fmt.Println("Output Hash:", outputHash)  // Add this line to log the output hash

    return outputHash, nil
}

func createTransactionFromRandomDataset() error {
    algoPath, err := fetchAlgorithmFromIPFS()
    if err != nil {
        fmt.Printf("Error fetching algorithm: %v\n", err)
        return fmt.Errorf("failed to fetch algorithm: %v", err)
    }

    datasetHashes := fetchDatasetHashes()
    if len(datasetHashes) == 0 {
        return fmt.Errorf("no datasets available")
    }

    datasetHash := pickRandomDataset(datasetHashes)

    datasetPath, err := fetchDatasetFromIPFS(datasetHash)
    if err != nil {
        fmt.Printf("Error fetching dataset: %v\n", err)
        return fmt.Errorf("failed to fetch dataset: %v", err)
    }

    outputHash, err := runAlgorithm(algoPath, datasetPath)
    if err != nil {
        fmt.Printf("Error running algorithm: %v\n", err)
        return fmt.Errorf("failed to run algorithm: %v", err)
    }
    

    transaction := Transaction{
        Sender:   "Node",
        Receiver: "Network",
        Metadata: map[string]string{
            "dataset_hash":   datasetHash,
            "algorithm_hash": ALGORITHM_HASH,
            "output_hash":    outputHash,
            "timestamp":      time.Now().Format(time.RFC3339),
        },
    }
    
    fmt.Println("Transaction", transaction);

    mu.Lock()
    Mempool = append(Mempool, transaction)
    mu.Unlock()

    err = sendTransactionToServer(transaction)
    if err != nil {
        fmt.Printf("Error sending transaction to server: %v\n", err)
        return fmt.Errorf("failed to send transaction to server: %v", err)
    }
    fmt.Println("Successfully sent transaction to server")

    return nil
}

func sendTransactionToServer(transaction Transaction) error {
	url := "http://localhost:5001/receiveTransaction" // The endpoint in server.go

	// Convert transaction to JSON
	data, err := json.Marshal(transaction)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction: %v", err)
	}

	// Send HTTP POST request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-OK response: %v", resp.Status)
	}

	return nil
}

func main() {
	rand.Seed(time.Now().UnixNano())

	err := initIPFSDesktop()
	if err != nil {
		fmt.Printf("Failed to connect to IPFS Desktop: %v\n", err)
		fmt.Println("Please make sure IPFS Desktop AppImage is running")
		return
	}

	http.HandleFunc("/processRandomDataset", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Received request to process random dataset...")
		
		err := createTransactionFromRandomDataset()
		if err != nil {
			fmt.Printf("Error creating transaction: %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		fmt.Println("Transaction successfully created and sent to server")
		fmt.Fprintf(w, "Transaction processed and added to mempool.")
	})

	// Add a simple health check endpoint
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Server is running!")
	})

	fmt.Println("Node running on port 5000...")
	if err := http.ListenAndServe(":5000", nil); err != nil {
		fmt.Printf("Server failed to start: %v\n", err)
	}
}
