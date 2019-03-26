package blockchain

import "crypto/sha256"

type MerkleTree struct {
	RootNode *MerkleNode
}

type MerkleNode struct {
	LeftNode  *MerkleNode
	RightNode *MerkleNode
	Data      []byte
}

func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	newNode := MerkleNode{}

	if left == nil && right == nil {
		hash := sha256.Sum256(data)
		newNode.Data = hash[:]
	} else {
		prevHash := append(left.Data, right.Data...)
		hash := sha256.Sum256(prevHash)
		newNode.Data = hash[:]
	}

	newNode.LeftNode = left
	newNode.RightNode = right

	return &newNode
}

func NewMerkleTree(data [][]byte) *MerkleTree {
	var nodes []MerkleNode

	if len(data)%2 != 0 {
		data = append(data, data[len(data)-1])

	}

	for _, leaf := range data {
		node := NewMerkleNode(nil, nil, leaf)
		nodes = append(nodes, *node)
	}

	for i := 0; i < len(data)/2; i++ {
		var level []MerkleNode

		for j := 0; j < len(nodes); j += 2 {
			node := NewMerkleNode(&nodes[j], &nodes[j+1], nil)
			level = append(level, *node)
		}

		nodes = level
	}

	tree := MerkleTree{&nodes[0]}

	return &tree
}
