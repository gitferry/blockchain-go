package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dgraph-io/badger"
)

const (
	dbPath = "./tmp/blocks_%s"
)

type BlockChain struct {
	LastHash []byte
	Database *badger.DB
}

type BlockChainIterator struct {
	CurrentHash []byte
	Database    *badger.DB
}

func DBexists(path string) bool {
	if _, err := os.Stat(path + "/MANEFEST"); err != nil {
		return false
	}

	return true
}

func retry(dir string, originalOpts badger.Options) (*badger.DB, error) {
	lockPath := filepath.Join(dir, "LOCK")

	if err := os.Remove(lockPath); err != nil {
		return nil, fmt.Errorf(`removing "LOCK": %s`, err)
	}

	retryOpts := originalOpts
	retryOpts.Truncate = true
	db, err := badger.Open(retryOpts)

	return db, err
}

func OpenDB(dir string, opts badger.Options) (*badger.DB, error) {
	if db, err := badger.Open(opts); err != nil {
		if strings.Contains(err.Error(), "LOCK") {
			if db, err := retry(dir, opts); err == nil {
				log.Println("database unlocked, value log truncated")
				return db, err
			}
			log.Println("Could not unlock the database:", err)
		}

		return nil, err
	} else {
		return db, nil
	}
}

func InitBlockchain(address string, nodeId string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeId)

	if isDBExist(path) {
		fmt.Println("Blockchain already exist")
		runtime.Goexit()
	}

	opts := badger.DefaultOptions
	opts.Dir = dbPath
	opts.ValueDir = dbPath

	db, err := OpenDB(path, opts)
	HandleErr(err)

	err = db.Update(func(txn *badger.Txn) error {
		cbtx := CoinBaseTx(address, genesisData)
		genesis := Genesis(cbtx)
		fmt.Println("Genesis created")
		err := txn.Set(genesis.Hash, genesis.Serialize())
		HandleErr(err)

		err = txn.Set([]byte("lh"), genesis.Hash)
		lastHash = genesis.Hash

		return err
	})

	HandleErr(err)

	blockchain := BlockChain{lastHash, db}
	return &blockchain
}

func ContinueBlockchain(nodeId string) *BlockChain {
	path := fmt.Sprintf(dbPath, nodeId)
	if isDBExist(path) == false {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit()
	}

	var lastHash []byte

	opts := badger.DefaultOptions
	opts.Dir = dbPath
	opts.ValueDir = dbPath

	db, err := OpenDB(path, opts)
	HandleErr(err)

	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		HandleErr(err)
		lastHash, err = item.Value()

		return err
	})
	HandleErr(err)

	blockchain := BlockChain{lastHash, db}

	return &blockchain
}

func (chain *BlockChain) FindUnspentOutputs() map[string]TxOutputs {
	UTXO := make(map[string]TxOutputs)
	spentTXOs := make(map[string][]int)

	iter := chain.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		outputs:
			for outIdx, out := range tx.Outputs {
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue outputs
						}
					}
				}

				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}

			if tx.IsCoinbase() == false {
				for _, in := range tx.Inputs {
					inTxID := hex.EncodeToString(in.ID)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Out)
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return UTXO
}

func (chain *BlockChain) MineBlock(transactions []*Transaction) *Block {
	var lastHash []byte
	var lastHeight int

	for _, tx := range transactions {
		if chain.VerifyTx(tx) != true {
			log.Panic("Invalid Transaction")
		}
	}

	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		HandleErr(err)
		lastHash, err = item.Value()

		item, err = txn.Get(lastHash)
		HandleErr(err)
		lastBlockData, _ := item.Value()

		lastBlock := Deserialize(lastBlockData)

		lastHeight := lastBlock.Height

		return err
	})
	HandleErr(err)
	newBlock := CreateBlock(transactions, lastHash, lastHeight+1)

	err = chain.Database.Update(func(txn *badger.Txn) error {
		err = txn.Set(newBlock.Hash, newBlock.Serialize())
		HandleErr(err)
		err := txn.Set([]byte("lh"), newBlock.Hash)

		chain.LastHash = newBlock.Hash

		return err
	})
	HandleErr(err)

	return newBlock
}

func (chain *BlockChain) AddBlock(block *Block) {
	err := chain.Database.Update(func(txn *badger.Txn) error {
		if _, err := txn.Get(block.Hash); err == nil {
			return nil
		}

		blockData := block.Serialize()
		err := txn.Set(block.Hash, blockData)
		HandleErr(err)

		item, err := txn.Get([]byte("lh"))
		HandleErr(err)
		lastHash, _ := item.Value()

		item, err = txn.Get(lastHash)
		HandleErr(err)
		lastBlockData, _ := item.Value()

		lastBlock := Deserialize(lastBlockData)

		if block.Height > lastBlock.Height {
			err := txn.Set([]byte("lh"), block.Hash)
			HandleErr(err)
			chain.LastHash = lastHash
		}

		return nil
	})

	HandleErr(err)
}

func (chain *BlockChain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	err := chain.Database.View(func(txn *badger.Txn) error {
		if item, err := txn.Get(blockHash); err != nil {
			return errors.New("Block is not found")
		} else {
			blockData, _ := item.Value()
			block = *Deserialize(blockData)
		}

		return nil
	})

	if err != nil {
		return block, err
	}
	return block, nil
}

func (chain *BlockChain) GetBlockHashes() [][]byte {
	var blockHashes [][]byte

	blockItr := chain.Iterator()

	for {
		block := blockItr.Next()

		blockHashes = append(blockHashes, block.Hash)

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return blockHashes
}

func (chain *BlockChain) GetBestHeight() int {
	var bestHeight int

	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		HandleErr(err)
		lastHash, _ := item.Value()

		item, err = txn.Get(lastHash)
		HandleErr(err)
		lastBlockData, _ := item.Value()

		lastBlock := *Deserialize(lastBlockData)
		bestHeight = lastBlock.Height

		return nil
	})

	HandleErr(err)

	return bestHeight
}

func (chain *BlockChain) Iterator() *BlockChainIterator {
	iter := BlockChainIterator{chain.LastHash, chain.Database}

	return &iter
}

func (iter *BlockChainIterator) Next() *Block {
	var block *Block

	err := iter.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(iter.CurrentHash)
		HandleErr(err)

		encodedBlock, err := item.Value()
		block = Deserialize(encodedBlock)

		return err
	})

	HandleErr(err)

	iter.CurrentHash = block.PrevHash

	return block
}

func (bc *BlockChain) FindTx(ID []byte) (Transaction, error) {
	iter := bc.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			if bytes.Compare(tx.ID, ID) == 0 {
				return *tx, nil
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction does not exist")
}

func (bc *BlockChain) SignTx(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTxs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		prevTx, err := bc.FindTx(in.ID)
		HandleErr(err)
		prevTxs[hex.EncodeToString(in.ID)] = prevTx
	}

	tx.Sign(privKey, prevTxs)
}

func (bc *BlockChain) VerifyTx(tx *Transaction) bool {
	if tx.IsCoinbase() {
		return true
	}

	prevTxs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		prevTx, err := bc.FindTx(in.ID)
		HandleErr(err)
		prevTxs[hex.EncodeToString(in.ID)] = prevTx
	}

	return tx.Verify(prevTxs)
}

func isDBExist() bool {
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		return false
	}

	return true
}
