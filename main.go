package main

import (
	"fmt"
	"strconv"

	"github.com/gitferry/blockchain-go/blockchain"
)

func main() {
	chain := blockchain.InitBlockchain()

	chain.AddBlock("The first block")
	chain.AddBlock("The second block")
	chain.AddBlock("The third block")

	for _, block := range chain.Blocks {
		fmt.Printf("Previous block hash: %x\n", block.PrevHash)
		fmt.Printf("Data in the block: %s\n", block.Data)
		fmt.Printf("Block hash: %x\n", block.Hash)

		pow := blockchain.NewProof(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()
	}
}
