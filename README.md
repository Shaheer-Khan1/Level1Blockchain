# IPFS-Based P2P Blockchain System

#Introduction
This project is a decentralized blockchain system that leverages IPFS for data storage and a peer-to-peer (P2P) network for transaction propagation and block mining. It ensures deterministic execution of algorithms and maintains a secure, distributed ledger across network nodes.

## Features
- **IPFS Integration**: Securely store and retrieve data and algorithms using IPFS content addressing.
- **P2P Network**: Enable real-time peer-to-peer communication where each node maintains the full blockchain and validates transactions.
- **Transaction Processing**: Transactions include input data, an algorithm reference, computation results, and cryptographic hashes for verification.
- **Block Mining**: Nodes aggregate transactions, mine blocks using a nonce to meet a difficulty target, and broadcast valid blocks to the network.

## Functional Modules
### IPFS Module
- Store algorithm source code and input data.
- Retrieve stored content using IPFS Content Identifier (CID).
- Uses `github.com/ipfs/go-ipfs-api` for IPFS interaction.

### P2P Network
- Establishes connections using `github.com/libp2p/go-libp2p`.
- Broadcasts transactions and blocks to all connected peers.
- Synchronizes blockchain state across the network.

### Blockchain Module
- Defines transaction and block structures.
- Implements transaction validation by re-executing algorithms on input data.
- Supports block mining using a proof-of-work (PoW) mechanism.

## Data Structures
### Transaction
```go
 type Transaction struct {
     InputData string
     Algorithm string
     Result string
     Hash string
 }
```

### Block
```go
 type Block struct {
     Transactions []Transaction
     PrevHash string
     Nonce int64
     Timestamp int64
     BlockHash string
 }
```

## Technical Requirements
### Programming Language
- Go

### Libraries Used
- `github.com/ipfs/go-ipfs-api` for IPFS operations.
- `github.com/libp2p/go-libp2p` for peer-to-peer networking.
- `crypto/sha256` for cryptographic hashing.
- `encoding/json` for serialization.

### Environment
- Ensure IPFS daemon is installed and running.
- Network connectivity for peer-to-peer communication.

## Installation and Setup
### Prerequisites
- Install [Go](https://go.dev/doc/install).
- Install [IPFS](https://docs.ipfs.io/install/).
- Set up the IPFS daemon:
  ```sh
  ipfs daemon
  ```

### Clone the Repository
```sh
git clone https://github.com/your-repo/ipfs-p2p-blockchain.git
cd ipfs-p2p-blockchain
```

### Install Dependencies
```sh
go mod tidy
```

### Run the System
```sh
go run main.go
```

## Future Enhancements
- Implement smart contract execution.
- Develop a front-end interface for monitoring blockchain activity.
- Support alternative consensus mechanisms like Proof of Stake (PoS).

## License
This project is licensed under the MIT License.

## Contributing
Contributions are welcome! Feel free to submit pull requests or raise issues for discussion.

