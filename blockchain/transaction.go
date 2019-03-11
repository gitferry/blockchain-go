package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
)

type Transaction struct {
	ID      []byte
	Inputs  []TxInput
	Outputs []TxOutput
}

type TxOutput struct {
	Value  int
	PubKey string
}

type TxInput struct {
	ID  []byte
	Out int
	Sig string
}

func CoinBaseTx(to, data string) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Coins to %s", to)
	}

	txInput := TxInput{[]byte{}, -1, data}
	txOutput := TxOutput{100, to}

	tx := Transaction{nil, []TxInput{txInput}, []TxOutput{txOutput}}
	tx.SetID()

	return &tx
}

func (tx *Transaction) SetID() {
	var encoded bytes.Buffer
	var hash [32]byte

	encode := gob.NewEncoder(&encoded)
	err := encode.Encode(tx)
	HandleErr(err)

	hash = sha256.Sum256(encoded.Bytes())
	tx.ID = hash[:]
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Inputs) == 1 && len(tx.Inputs[0].ID) == 0 && tx.Inputs[0].Out == -1
}

func (input *TxInput) CanUnlock(data string) bool {
	return input.Sig == data
}

func (output *TxOutput) CanBeUnlocked(data string) bool {
	return output.PubKey == data
}

func NewTransaction(from string, to string, value int, chain *BlockChain) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput

	acc, validOutputs := chain.FindSpendableOutputs(from, value)

	if acc < value {
		log.Panic("Error: not enough funds")
	}

	for txid, outputs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		HandleErr(err)
		for _, out := range outputs {
			input := TxInput{txID, out, from}
			inputs = append(inputs, input)
		}
	}

	outputs = append(outputs, TxOutput{value, to})

	if acc > value {
		output := TxOutput{acc - value, from}
		outputs = append(outputs, output)
	}

	tx := Transaction{nil, inputs, outputs}
	tx.SetID()

	return &tx
}

func (tx *Transaction) PrintTx() {
	inputs := tx.Inputs
	fmt.Printf("Print tx %s:\n", hex.EncodeToString(tx.ID))
	for idx, input := range inputs {
		fmt.Printf("input %d:\n", idx)
		fmt.Printf("tx id: %s, tx index: %d, tx address: %s\n", hex.EncodeToString(input.ID), input.Out, input.Sig)
	}

	outputs := tx.Outputs
	for idx, output := range outputs {
		fmt.Printf("output %d\n", idx)
		fmt.Printf("out value: %d, out address: %s\n", output.Value, output.PubKey)
	}
}
