package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

var (
	Mempool []Transaction // Shared mempool for transactions
	mu      sync.Mutex    // Mutex to synchronize access to the mempool
)

type Transaction struct {
	Sender   string            `json:"sender"`
	Receiver string            `json:"receiver"`
	Metadata map[string]string `json:"metadata"`
}

// Fetches a dataset from IPFS using its CID
func fetchDatasetFromIPFS(cid string) string {
	ipfsNodeURL := "http://localhost:5001/api/v0/cat?arg=" + cid
	resp, err := http.Get(ipfsNodeURL)
	if err != nil {
		fmt.Println("Error fetching dataset from IPFS:", err)
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Failed to fetch dataset. HTTP status:", resp.Status)
		return ""
	}

	// Create a local file to store the fetched dataset
	localPath := "/path/to/local/copy/of/" + cid
	file, err := os.Create(localPath)
	if err != nil {
		fmt.Println("Error creating local file:", err)
		return ""
	}
	defer file.Close()

	// Copy the data from the response to the local file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		fmt.Println("Error saving dataset to local file:", err)
		return ""
	}

	fmt.Println("Dataset saved locally at:", localPath)
	return localPath
}

// Simulates running an algorithm on a dataset
func runAlgorithm(datasetPath string) string {
	fmt.Println("Running algorithm on dataset:", datasetPath)
	return fmt.Sprintf("output_hash_of_%s", datasetPath)
}

func pickRandomDataset(hashes []string) string {
	if len(hashes) == 0 {
		return ""
	}
	return hashes[rand.Intn(len(hashes))]
}

func main() {
	// Example usage
	cid := "QmXDSyMuWipxBTatdD2go78zx297CU75ysrEKvuzvwRVQC"
	localPath := fetchDatasetFromIPFS(cid)
	if localPath != "" {
		output := runAlgorithm(localPath)
		fmt.Println("Algorithm output:", output)
	}
}

