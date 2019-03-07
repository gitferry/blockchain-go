package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/gitferry/blockchain-go/blockchain"
)

type CommandLine struct {
	blockchain *blockchain.BlockChain
}

func (cli *CommandLine) PrintUsage() {
	fmt.Println("Usage:")
	fmt.Println(" add -block BLOCK_DATA - add a block to the blockchain")
	fmt.Println(" print - Prints the blocks in the chain")
}

func (cli *CommandLine) ValidateArgs() {
	if len(os.Args) < 2 {
		cli.PrintUsage()
		runtime.Goexit()
	}
}

func (cli *CommandLine) AddBlock(data string) {
	cli.blockchain.AddBlock(data)
	fmt.Println("Block added!")
}

func (cli *CommandLine) PrintChain() {
	iter := cli.blockchain.Iterator()

	for {
		block := iter.Next()
		fmt.Printf("Previous block hash: %x\n", block.PrevHash)
		fmt.Printf("Data in the block: %s\n", block.Data)
		fmt.Printf("Block hash: %x\n", block.Hash)

		pow := blockchain.NewProof(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		fmt.Println()

		if len(block.PrevHash) == 0 {
			break
		}
	}
}

func (cli *CommandLine) Run() {
	cli.ValidateArgs()

	addBlockcmd := flag.NewFlagSet("add", flag.ExitOnError)
	printChaincmd := flag.NewFlagSet("print", flag.ExitOnError)
	addBlockData := addBlockcmd.String("block", "", "Block data")

	switch os.Args[1] {
	case "add":
		err := addBlockcmd.Parse(os.Args[2:])
		blockchain.HandleErr(err)
	case "print":
		err := printChaincmd.Parse(os.Args[2:])
		blockchain.HandleErr(err)
	default:
		cli.PrintUsage()
		runtime.Goexit()
	}

	if addBlockcmd.Parsed() {
		if *addBlockData == "" {
			cli.PrintUsage()
			runtime.Goexit()
		}
		cli.AddBlock(*addBlockData)
	}

	if printChaincmd.Parsed() {
		cli.PrintChain()
	}
}

func main() {
	defer os.Exit(0)
	chain := blockchain.InitBlockchain()
	defer chain.Database.Close()

	cli := CommandLine{chain}
	cli.Run()
}
