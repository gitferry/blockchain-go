package cli

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/gitferry/blockchain-go/wallet"

	"github.com/gitferry/blockchain-go/blockchain"
)

type CommandLine struct{}

func (cli *CommandLine) PrintUsage() {
	fmt.Println("Usage:")
	fmt.Println(" getbalance -address ADRESS - get the balance for that address")
	fmt.Println(" createblockchain -address ADRESS creates a blockchain and that address mines the genessis block")
	fmt.Println(" print - Prints the blocks in the chain")
	fmt.Println(" sent -from FROM -to To -amount AMOUNT - send amount of tokens")
	fmt.Println(" createwallet - Create a new wallet")
	fmt.Println(" listaddresses - Lists the addresses in our wallet file")
}

func (cli *CommandLine) ValidateArgs() {
	if len(os.Args) < 2 {
		cli.PrintUsage()
		runtime.Goexit()
	}
}

func (cli *CommandLine) PrintChain() {
	chain := blockchain.ContinueBlockchain("")
	defer chain.Database.Close()
	iter := chain.Iterator()

	for {
		block := iter.Next()

		fmt.Printf("Previous block hash: %x\n", block.PrevHash)
		fmt.Printf("Block hash: %x\n", block.Hash)
		pow := blockchain.NewProof(block)
		fmt.Printf("PoW: %s\n", strconv.FormatBool(pow.Validate()))
		for _, tx := range block.Transactions {
			fmt.Println(tx)
		}
		fmt.Println()

		if len(block.PrevHash) == 0 {
			break
		}
	}
}

func (cli *CommandLine) CreateBlockChain(address string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.InitBlockchain(address)
	chain.Database.Close()
	fmt.Println("A blockchain created!")
}

func (cli *CommandLine) GetBalance(address string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.ContinueBlockchain(address)
	defer chain.Database.Close()

	balance := 0
	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	UTXOs := chain.FindUTXO(pubKeyHash)

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of %s is: %d\n", address, balance)
}

func (cli *CommandLine) Send(from, to string, amount int) {
	if !wallet.ValidateAddress(from) {
		log.Panic("Address is not valid")
	}
	if !wallet.ValidateAddress(to) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.ContinueBlockchain(from)
	defer chain.Database.Close()

	tx := blockchain.NewTransaction(from, to, amount, chain)
	chain.AddBlock([]*blockchain.Transaction{tx})
	fmt.Println("Success!")
}

func (cli *CommandLine) NewWallet() {
	wallets, _ := wallet.CreateWalltes()
	address := wallets.AddWallet()
	wallets.SaveFile()

	fmt.Printf("New wallet address is :%s", address)
}

func (cli *CommandLine) ListAddresses() {
	wallets, _ := wallet.CreateWalltes()

	addresses := wallets.GetAllAddresses()

	for idx, address := range addresses {
		fmt.Printf("%d. %s\n", idx, address)
	}
}

func (cli *CommandLine) Run() {
	cli.ValidateArgs()

	getBalancecmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	createBlockchaincmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	sendcmd := flag.NewFlagSet("send", flag.ExitOnError)
	printChaincmd := flag.NewFlagSet("print", flag.ExitOnError)
	createWalletcmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
	listAddressescmd := flag.NewFlagSet("listaddresses", flag.ExitOnError)

	getBalanceAddress := getBalancecmd.String("address", "", "The address")
	createBlockchainAddress := createBlockchaincmd.String("address", "", "The address")
	sendFrom := sendcmd.String("from", "", "address sent from")
	sendTo := sendcmd.String("to", "", "address sent to")
	sendAmount := sendcmd.Int("amount", 0, "amount sent to")

	switch os.Args[1] {
	case "getbalance":
		err := getBalancecmd.Parse(os.Args[2:])
		blockchain.HandleErr(err)
	case "createblockchain":
		err := createBlockchaincmd.Parse(os.Args[2:])
		blockchain.HandleErr(err)
	case "send":
		err := sendcmd.Parse(os.Args[2:])
		blockchain.HandleErr(err)
	case "print":
		err := printChaincmd.Parse(os.Args[2:])
		blockchain.HandleErr(err)
	case "createwallet":
		err := createWalletcmd.Parse(os.Args[2:])
		blockchain.HandleErr(err)
	case "listaddresses":
		err := listAddressescmd.Parse(os.Args[2:])
		blockchain.HandleErr(err)
	default:
		cli.PrintUsage()
		runtime.Goexit()
	}

	if getBalancecmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalancecmd.Usage()
			runtime.Goexit()
		}
		cli.GetBalance(*getBalanceAddress)
	}

	if createBlockchaincmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockchaincmd.Usage()
			runtime.Goexit()
		}
		cli.CreateBlockChain(*createBlockchainAddress)
	}

	if sendcmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendcmd.Usage()
			runtime.Goexit()
		}
		cli.Send(*sendFrom, *sendTo, *sendAmount)
	}

	if printChaincmd.Parsed() {
		cli.PrintChain()
	}

	if createWalletcmd.Parsed() {
		cli.NewWallet()
	}

	if listAddressescmd.Parsed() {
		cli.ListAddresses()
	}
}
