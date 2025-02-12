package main

import "time"

type Transaction struct {
    Sender    string
    Receiver  string
    Metadata  map[string]string
}

type Block struct {
    Index        int
    Timestamp    time.Time
    Transactions []Transaction
    PreviousHash string
    Hash         string
    Nonce        int
}

var Blockchain []Block
var Mempool []Transaction

