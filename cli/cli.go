package cli

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/gitferry/blockchain-go/network"

	"github.com/gitferry/blockchain-go/blockchain"
	"github.com/gitferry/blockchain-go/wallet"
)

type CommandLine struct{}

func (cli *CommandLine) PrintUsage() {
	fmt.Println("Usage:")
	fmt.Println(" getbalance -address ADRESS - get the balance for that address")
	fmt.Println(" createblockchain -address ADRESS creates a blockchain and that address mines the genessis block")
	fmt.Println(" print - Prints the blocks in the chain")
	fmt.Println(" sent -from FROM -to To -amount AMOUNT -mine - send amount of tokens. Then -mine flag is set")
	fmt.Println(" createwallet - Create a new wallet")
	fmt.Println(" listaddresses - Lists the addresses in our wallet file")
	fmt.Println(" reindexutxo - Rebuild the UTXO set")
	fmt.Println(" startnode -miner ADDRESS - Start a node with ID specified in NODE_ID env. var. -mine enables mining")
}

func (cli *CommandLine) ValidateArgs() {
	if len(os.Args) < 2 {
		cli.PrintUsage()
		runtime.Goexit()
	}
}

func (cli *CommandLine) PrintChain(nodeId string) {
	chain := blockchain.ContinueBlockchain(nodeId)
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

func (cli *CommandLine) CreateBlockChain(address, nodeId string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.InitBlockchain(address, nodeId)
	defer chain.Database.Close()

	UTXOSet := blockchain.UTXOSet{chain}
	UTXOSet.Reindex()
	fmt.Println("A blockchain created!")
}

func (cli *CommandLine) StartNode(nodeId, minerAddress string) {
	fmt.Printf("Starting Node %s\n", nodeId)

	if len(minerAddress) > 0 {
		if wallet.ValidateAddress(minerAddress) {
			fmt.Printf("Mining is on. Address to receive rewards: ", minerAddress)
		} else {
			log.Panic("Wrong miner address!")
		}
	}

	network.StartServer(nodeId, minerAddress)
}

func (cli *CommandLine) GetBalance(address, nodeId string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.ContinueBlockchain(nodeId)
	utxoSet := blockchain.UTXOSet{chain}
	defer chain.Database.Close()

	balance := 0
	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	UTXOs := utxoSet.FindUTXO(pubKeyHash)

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("Balance of %s is: %d\n", address, balance)
}

func (cli *CommandLine) Send(from, to string, amount int, nodeId string, mineNow bool) {
	if !wallet.ValidateAddress(from) {
		log.Panic("Address is not valid")
	}
	if !wallet.ValidateAddress(to) {
		log.Panic("Address is not valid")
	}
	chain := blockchain.ContinueBlockchain(nodeId)
	utxoSet := blockchain.UTXOSet{chain}
	defer chain.Database.Close()

	wallets, err := wallet.CreateWalltes(nodeId)
	if err != nil {
		log.Panic()
	}
	wallet := wallets.GetWallet(from)

	tx := blockchain.NewTransaction(&wallet, to, amount, &utxoSet)
	if mineNow {
		cbTx := blockchain.CoinBaseTx(from, "")
		txs := []*blockchain.Transaction{cbTx, tx}
		block := chain.MineBlock(txs)
		utxoSet.Update(block)
	} else {
		network.SendTx(network.KnownNodes[0], tx)
		fmt.Println("send tx")
	}

	fmt.Println("Success!")
}

func (cli *CommandLine) reindexUTXO(nodeId string) {
	chain := blockchain.ContinueBlockchain(nodeId)
	defer chain.Database.Close()
	UTXOSet := blockchain.UTXOSet{chain}
	UTXOSet.Reindex()

	count := UTXOSet.CountTransactions()
	fmt.Printf("Done! There are %d transactions in the UTXO set.\n", count)
}

func (cli *CommandLine) NewWallet(nodeId string) {
	wallets, _ := wallet.CreateWalltes(nodeId)
	address := wallets.AddWallet()
	wallets.SaveFile(nodeId)

	fmt.Printf("New wallet address is :%s", address)
}

func (cli *CommandLine) ListAddresses(nodeId string) {
	wallets, _ := wallet.CreateWalltes(nodeId)

	addresses := wallets.GetAllAddresses()

	for idx, address := range addresses {
		fmt.Printf("%d. %s\n", idx, address)
	}
}

func (cli *CommandLine) Run() {
	cli.ValidateArgs()

	nodeId := os.Getenv("NODE_ID")
	if nodeId == "" {
		fmt.Printf("NODE_ID env is not set!")
		runtime.Goexit()
	}

	getBalancecmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	createBlockchaincmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	sendcmd := flag.NewFlagSet("send", flag.ExitOnError)
	printChaincmd := flag.NewFlagSet("print", flag.ExitOnError)
	createWalletcmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
	listAddressescmd := flag.NewFlagSet("listaddresses", flag.ExitOnError)
	reindexUTXOcmd := flag.NewFlagSet("reindexutxo", flag.ExitOnError)
	startNodecmd := flag.NewFlagSet("startnode", flag.ExitOnError)

	getBalanceAddress := getBalancecmd.String("address", "", "The address")
	createBlockchainAddress := createBlockchaincmd.String("address", "", "The address")
	sendFrom := sendcmd.String("from", "", "address sent from")
	sendTo := sendcmd.String("to", "", "address sent to")
	sendAmount := sendcmd.Int("amount", 0, "amount sent to")
	sendMine := sendcmd.Bool("mine", false, "Mine immediately on the same node")
	startNodeMiner := startNodecmd.String("miner", "", "Enable mining mode and send reward to the miner.")

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
	case "reindexutxo":
		err := reindexUTXOcmd.Parse(os.Args[2:])
		blockchain.HandleErr(err)
	case "startnode":
		err := startNodecmd.Parse(os.Args[2:])
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
		cli.GetBalance(*getBalanceAddress, nodeId)
	}

	if createBlockchaincmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockchaincmd.Usage()
			runtime.Goexit()
		}
		cli.CreateBlockChain(*createBlockchainAddress, nodeId)
	}

	if sendcmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendcmd.Usage()
			runtime.Goexit()
		}
		cli.Send(*sendFrom, *sendTo, *sendAmount, nodeId, *sendMine)
	}

	if printChaincmd.Parsed() {
		cli.PrintChain(nodeId)
	}

	if createWalletcmd.Parsed() {
		cli.NewWallet(nodeId)
	}

	if listAddressescmd.Parsed() {
		cli.ListAddresses(nodeId)
	}

	if reindexUTXOcmd.Parsed() {
		cli.reindexUTXO(nodeId)
	}

	if startNodecmd.Parsed() {
		nodeId := os.Getenv("NODE_ID")
		if nodeId == "" {
			startNodecmd.Usage()
			runtime.Goexit()
		}
		cli.StartNode(nodeId, *startNodeMiner)
	}
}
