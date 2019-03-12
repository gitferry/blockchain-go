package main

import (
	"os"

	"github.com/gitferry/blockchain-go/cli"
	"github.com/gitferry/blockchain-go/wallet"
)

func main() {
	defer os.Exit(0)
	cli := cli.CommandLine{}
	cli.Run()

	w := wallet.MakeWallet()
	w.Address()
}
