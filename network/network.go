package network

import (
	"bytes"
	"encoding/gob"
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

func SendBlocks(address string) {
	payload := GobEncoder(GetBlocks{nodeAddress})

	request := append(CmdToBytes("getblocks"), payload...)

	SendData(address, request)
}

func SendGetData(address, kind string, id []byte) {
	payload := GobEncoder(GetData{nodeAddress, kind, id})

	request := append(CmdToBytes("getdata"), payload...)

	SendData(address, request)
}
