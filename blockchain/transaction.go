package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/gitferry/blockchain-go/wallet"
)

type Transaction struct {
	ID      []byte
	Inputs  []TxInput
	Outputs []TxOutput
}

func (tx *Transaction) Serialize() []byte {
	var encoded bytes.Buffer

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx)
	if err != nil {
		log.Panic(err)
	}

	return encoded.Bytes()
}

func DeserializeTransaction(data []byte) Transaction {
	var transaction Transaction

	decoder := gob.NewDecoder(bytes.NewReader(data))
	err := decoder.Decode(&transaction)
	HandleErr(err)
	return transaction
}

func (tx *Transaction) Hash() []byte {
	var hash [32]byte

	TxCopy := *tx
	TxCopy.ID = []byte{}

	hash = sha256.Sum256(TxCopy.Serialize())

	return hash[:]
}

func (tx *Transaction) Sign(privKey ecdsa.PrivateKey, prevTXs map[string]Transaction) {
	if tx.IsCoinbase() {
		return
	}

	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID == nil {
			log.Panic("ERROR: Previous transaction does not exit!")
		}
	}

	txCopy := tx.TrimmedCopy()

	for inIdx, in := range txCopy.Inputs {
		prevTX := prevTXs[hex.EncodeToString(in.ID)]
		in.Signature = nil
		in.PubKey = prevTX.Outputs[in.Out].PubKeyHash
		txCopy.ID = txCopy.Hash()
		in.PubKey = nil

		r, s, err := ecdsa.Sign(rand.Reader, &privKey, txCopy.ID)
		HandleErr(err)
		signature := append(r.Bytes(), s.Bytes()...)

		tx.Inputs[inIdx].Signature = signature
	}
}

func (tx *Transaction) TrimmedCopy() Transaction {
	var inputs []TxInput
	var outputs []TxOutput

	for _, in := range tx.Inputs {
		inputs = append(inputs, TxInput{in.ID, in.Out, nil, nil})
	}

	for _, out := range tx.Outputs {
		outputs = append(outputs, TxOutput{out.Value, out.PubKeyHash})
	}

	txCopy := Transaction{tx.ID, inputs, outputs}

	return txCopy
}

func (tx *Transaction) Verify(prevTXs map[string]Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	for _, in := range tx.Inputs {
		if prevTXs[hex.EncodeToString(in.ID)].ID != nil {
			log.Panic("ERROR: Previous transaction does not exit!")
		}
	}

	txCopy := tx.TrimmedCopy()
	curve := elliptic.P256()

	for _, in := range txCopy.Inputs {
		prevTX := prevTXs[hex.EncodeToString(in.ID)]
		in.Signature = nil
		in.PubKey = prevTX.Outputs[in.Out].PubKeyHash
		txCopy.ID = txCopy.Hash()
		in.PubKey = nil

		r := big.Int{}
		s := big.Int{}
		sigLen := len(in.Signature)
		r.SetBytes(in.Signature[:(sigLen / 2)])
		s.SetBytes(in.Signature[(sigLen / 2):])

		x := big.Int{}
		y := big.Int{}
		keyLen := len(in.PubKey)
		x.SetBytes(in.PubKey[:(keyLen / 2)])
		y.SetBytes(in.PubKey[(keyLen / 2):])

		rawPubKey := ecdsa.PublicKey{curve, &x, &y}
		if ecdsa.Verify(&rawPubKey, txCopy.ID, &r, &s) == false {
			return false
		}
	}

	return true
}

func (tx *Transaction) String() string {
	var lines []string

	lines = append(lines, fmt.Sprintf("--- Transaction %x:", tx.ID))
	for i, in := range tx.Inputs {
		lines = append(lines, fmt.Sprintf("    Input: %d", i))
		lines = append(lines, fmt.Sprintf("        TXID: %x", in.ID))
		lines = append(lines, fmt.Sprintf("        Out: %d", in.Out))
		lines = append(lines, fmt.Sprintf("        Signature: %x", in.Signature))
		lines = append(lines, fmt.Sprintf("        PubKey: %x", in.PubKey))
	}

	for i, out := range tx.Outputs {
		lines = append(lines, fmt.Sprintf("    Output: %d", i))
		lines = append(lines, fmt.Sprintf("        Value: %d", out.Value))
		lines = append(lines, fmt.Sprintf("        Script: %x", out.PubKeyHash))
	}

	return strings.Join(lines, "\n")
}

func CoinBaseTx(to, data string) *Transaction {
	if data == "" {
		randData := make([]byte, 24)
		_, err := rand.Read(randData)
		HandleErr(err)
		data = fmt.Sprintf("%x", randData)
	}

	txInput := TxInput{[]byte{}, -1, nil, []byte(data)}
	txOutput := NewTXOutput(20, to)

	tx := Transaction{nil, []TxInput{txInput}, []TxOutput{*txOutput}}
	tx.ID = tx.Hash()

	return &tx
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Inputs) == 1 && len(tx.Inputs[0].ID) == 0 && tx.Inputs[0].Out == -1
}

func NewTransaction(w *wallet.Wallet, to string, value int, UTXO *UTXOSet) *Transaction {
	var inputs []TxInput
	var outputs []TxOutput

	pubKeyHash := wallet.PublicKeyHash(w.PublicKey)

	acc, validOutputs := UTXO.FindSpendableOutputs(pubKeyHash, value)

	if acc < value {
		log.Panic("Error: not enough funds")
	}

	for txid, outputs := range validOutputs {
		txID, err := hex.DecodeString(txid)
		HandleErr(err)
		for _, out := range outputs {
			input := TxInput{txID, out, nil, w.PublicKey}
			inputs = append(inputs, input)
		}
	}

	from := fmt.Sprintf("%s", w.Address)

	outputs = append(outputs, *NewTXOutput(value, to))

	if acc > value {
		outputs = append(outputs, *NewTXOutput(acc-value, from))
	}

	tx := Transaction{nil, inputs, outputs}

	tx.ID = tx.Hash()
	UTXO.Blockchain.SignTx(&tx, w.PrivateKey)

	return &tx
}
