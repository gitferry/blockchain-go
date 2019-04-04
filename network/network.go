package network

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"runtime"
	"syscall"

	"github.com/gitferry/blockchain-go/blockchain"
	"gopkg.in/vrecan/death.v3"
)

const (
	protocol          = "tcp"
	version           = 1
	commandLineLength = 12
)

var (
	nodeAddress     string
	minerAddress    string
	KnownNodes      = []string{"localhost:3000"}
	blocksInTransit = [][]byte{}
	memoryPool      = make(map[string]blockchain.Transaction)
)

type Addr struct {
	AddrList []string
}

type Block struct {
	AddrFrom string
	Block    []byte
}

type GetBlocks struct {
	AddrFrom string
}

type GetData struct {
	AddrFrom string
	Type     string
	ID       []byte
}

type Inv struct {
	AddrFrom string
	Type     string
	Items    [][]byte
}

type Tx struct {
	AddrFrom    string
	Transaction []byte
}

type Version struct {
	Version    int
	BestHeight int
	AddrFrom   string
}

func CmdToBytes(cmd string) []byte {
	var bytes [commandLineLength]byte

	for i, c := range cmd {
		bytes[i] = byte(c)
	}

	return bytes[:]
}

func BytesToCmd(bytes []byte) string {
	var cmd []byte

	for _, b := range bytes {
		if b != 0x0 {
			cmd = append(bytes, b)
		}
	}

	return fmt.Sprintf("%s", cmd)
}

func CloseDB(chain *blockchain.BlockChain) {
	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	d.WaitForDeathWithFunc(func() {
		defer os.Exit(1)
		defer runtime.Goexit()
		chain.Database.Close()
	})
}

func GobEncoder(data interface{}) []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data)
	if err != nil {
		log.Panic(err)
	}

	return buff.Bytes()
}

func HandleConnection(conn net.Conn, chain *blockchain.BlockChain) {
	req, err := ioutil.ReadAll(conn)
	defer conn.Close()

	if err != nil {
		log.Panic(err)
	}
	command := BytesToCmd(req[:commandLineLength])
	fmt.Printf("Received %s command", command)

	switch command {
	case "addr":
		HandleAddr(req)
	case "block":
		HandleBlock(req, chain)
	case "tx":
		HandleTx(req, chain)
	case "inv":
		HandleInv(req, chain)
	case "getblocks":
		HandleGetBlocks(req, chain)
	case "getData":
		HandleGetData(req, chain)
	case "version":
		HandleVersion(req, chain)
	default:
		fmt.Println("Unknown command")

	}
}

func SendData(address string, data []byte) {
	conn, err := net.Dial(protocol, address)
	defer conn.Close()

	if err != nil {
		fmt.Println("%s is not available")

		var updatedKnownNodes []string

		for _, knownAddress := range KnownNodes {
			if knownAddress != address {
				updatedKnownNodes = append(updatedKnownNodes, knownAddress)
			}
		}

		return
	}

	_, err = io.Copy(conn, bytes.NewReader(data))
	if err != nil {
		log.Panic(err)
	}
}

func SendAddr(address string) {
	nodes := Addr{KnownNodes}
	nodes.AddrList = append(nodes.AddrList, nodeAddress)
	payload := GobEncoder(nodes)
	request := append(CmdToBytes("addr"), payload...)

	SendData(address, request)
}

func SendBlock(address string, b *blockchain.Block) {
	data := Block{nodeAddress, b.Serialize()}
	payload := GobEncoder(data)
	request := append(CmdToBytes("block"), payload...)

	SendData(address, request)
}

func SendInv(address, kind string, items [][]byte) {
	inventory := Inv{nodeAddress, kind, items}
	payload := GobEncoder(inventory)
	request := append(CmdToBytes("inv"), payload...)

	SendData(address, request)
}

func SendTx(address string, tx *blockchain.Transaction) {
	transaction := Tx{nodeAddress, tx.Serialize()}
	payload := GobEncoder(transaction)
	request := append(CmdToBytes("tx"), payload...)

	SendData(address, request)
}

func SendVersion(address string, chain *blockchain.BlockChain) {
	bestHeight := chain.GetBestHeight()

	v := Version{version, bestHeight, nodeAddress}
	payload := GobEncoder(v)
	request := append(CmdToBytes("version"), payload...)

	SendData(address, request)
}

func SendGetBlocks(address string) {
	payload := GobEncoder(GetBlocks{nodeAddress})

	request := append(CmdToBytes("getblocks"), payload...)

	SendData(address, request)
}

func SendGetData(address, kind string, id []byte) {
	payload := GobEncoder(GetData{nodeAddress, kind, id})

	request := append(CmdToBytes("getdata"), payload...)

	SendData(address, request)
}

func RequestBlocks() {
	for _, node := range KnownNodes {
		SendGetBlocks(node)
	}
}

func HandleAddr(request []byte) {
	var buffer bytes.Buffer
	var payload Addr

	buffer.Write(request[commandLineLength:])
	decode := gob.NewDecoder(&buffer)
	err := decode.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	KnownNodes = append(KnownNodes, payload.AddrList...)
	fmt.Printf("There are %d known nodes", len(KnownNodes))
	RequestBlocks()
}

func HandleBlock(request []byte, chain *blockchain.BlockChain) {
	var buffer bytes.Buffer
	var payload Block

	buffer.Write(request[commandLineLength:])
	decode := gob.NewDecoder(&buffer)
	err := decode.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	block := blockchain.Deserialize(payload.Block)

	fmt.Println("Received a new block!")
	chain.AddBlock(block)
	fmt.Printf("Added block %x\n", block.Hash)

	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		SendGetData(payload.AddrFrom, "block", blockHash)
		blockHash = blockHash[1:]
	} else {
		UTXOSet := blockchain.UTXOSet{chain}
		UTXOSet.Reindex()
	}

}

func HandleGetBlocks(request []byte, chain *blockchain.BlockChain) {
	var buffer bytes.Buffer
	var payload GetBlocks

	buffer.Write(request[commandLineLength:])
	decode := gob.NewDecoder(&buffer)
	err := decode.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	blocks := chain.GetBlockHashes()
	SendInv(payload.AddrFrom, "block", blocks)
}

func HandleGetData(request []byte, chain *blockchain.BlockChain) {
	var buffer bytes.Buffer
	var payload GetData

	buffer.Write(request[commandLineLength:])
	decode := gob.NewDecoder(&buffer)
	err := decode.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	if payload.Type == "block" {
		block, err := chain.GetBlock([]byte(payload.ID))
		if err != nil {
			log.Panic(err)
		}

		SendBlock(payload.AddrFrom, &block)
	}

	if payload.Type == "tx" {
		txID := hex.EncodeToString(payload.ID)
		tx := memoryPool[txID]
		SendTx(payload.AddrFrom, &tx)
	}
}

func NodeIsKnown(address string) bool {
	for _, node := range KnownNodes {
		if node == address {
			return true
		}
	}

	return false
}

func HandleVersion(request []byte, chain *blockchain.BlockChain) {
	var buffer bytes.Buffer
	var payload Version

	buffer.Write(request[commandLineLength:])
	decode := gob.NewDecoder(&buffer)
	err := decode.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	bestHeight := chain.GetBestHeight()
	otherBestHeight := payload.BestHeight

	if bestHeight < otherBestHeight {
		SendGetBlocks(payload.AddrFrom)
	} else if bestHeight > otherBestHeight {
		SendVersion(payload.AddrFrom, chain)
	}

	if !NodeIsKnown(payload.AddrFrom) {
		KnownNodes = append(KnownNodes, payload.AddrFrom)
	}
}

func MineTx(chain *blockchain.BlockChain) {
	var txs []*blockchain.Transaction

	for id, tx := range memoryPool {
		fmt.Printf("tx: %s\n", id)
		if chain.VerifyTx(&tx) == true {
			txs = append(txs, &tx)
		}
	}

	if len(txs) == 0 {
		fmt.Println("No validate tx")
		return
	}

	cbTx := blockchain.CoinBaseTx(nodeAddress, "")
	txs = append(txs, cbTx)

	newBlock := chain.MineBlock(txs)
	UTXOSet := blockchain.UTXOSet{chain}
	UTXOSet.Reindex()

	fmt.Println("New block mined")

	for _, tx := range txs {
		txID := hex.EncodeToString(tx.ID)
		delete(memoryPool, txID)
	}

	for _, node := range KnownNodes {
		if node != nodeAddress {
			SendInv(node, "block", [][]byte{newBlock.Hash})
		}
	}

	if len(memoryPool) > 0 {
		MineTx(chain)
	}
}

func HandleTx(request []byte, chain *blockchain.BlockChain) {
	var buffer bytes.Buffer
	var payload Tx

	buffer.Write(request[commandLineLength:])
	decode := gob.NewDecoder(&buffer)
	err := decode.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	transaction := blockchain.DeserializeTransaction(payload.Transaction)
	fmt.Println("Received a new transaction!")
	memoryPool[hex.EncodeToString(transaction.ID)] = transaction
	fmt.Printf("Added transaction %x, there are %d transactions in the memory pool\n", transaction.ID, len(memoryPool))

	if nodeAddress == KnownNodes[0] {
		for _, node := range KnownNodes {
			if node != nodeAddress && node != payload.AddrFrom {
				SendInv(node, "tx", [][]byte{transaction.ID})
			}
		}
	} else {
		if len(memoryPool) > 2 && len(minerAddress) > 0 {
			MineTx(chain)
		}
	}
}

func HandleInv(request []byte, chain *blockchain.BlockChain) {
	var buffer bytes.Buffer
	var payload Inv

	buffer.Write(request[commandLineLength:])
	decode := gob.NewDecoder(&buffer)
	err := decode.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("Received inventory %d %s\n", payload.Items, payload.Type)

	if payload.Type == "block" {
		blocksInTransit = payload.Items

		blockHash := payload.Items[0]
		SendGetData(payload.AddrFrom, "block", blockHash)

		blocksInTransit = blocksInTransit[1:]
	}

	if payload.Type == "tx" {
		txID := payload.Items[0]

		if memoryPool[hex.EncodeToString(txID)].ID == nil {
			SendGetData(payload.AddrFrom, "tx", txID)
		}
	}
}

func StartServer(nodeID, minerAddress string) {
	nodeAddress := fmt.Sprintf("localhost:%s", nodeID)
	minerAddress = minerAddress
	ln, err := net.Listen(protocol, nodeAddress)
	if err != nil {
		log.Panic(err)
	}
	defer ln.Close()

	chain := blockchain.ContinueBlockchain(nodeID)
	defer chain.Database.Close()
	go CloseDB(chain)

	if nodeAddress != KnownNodes[0] {
		SendVersion(nodeAddress, chain)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Panic()
		}

		go HandleConnection(conn, chain)
	}
}
