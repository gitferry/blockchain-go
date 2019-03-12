package blockchain

type TxOutput struct {
	Value  int
	PubKey string
}

type TxInput struct {
	ID  []byte
	Out int
	Sig string
}

func (input *TxInput) CanUnlock(data string) bool {
	return input.Sig == data
}

func (output *TxOutput) CanBeUnlocked(data string) bool {
	return output.PubKey == data
}
