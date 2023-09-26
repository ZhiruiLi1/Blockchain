package wallet

import (
	"Coin/pkg/block"
	"Coin/pkg/blockchain/chainwriter"
	"Coin/pkg/id"
)

// CoinInfo holds the information about a TransactionOutput
// necessary for making a TransactionInput.
// ReferenceTransactionHash is the hash of the transaction that the
// output is from.
// OutputIndex is the index into the Outputs array of the
// Transaction that the TransactionOutput is from.
// TransactionOutput is the actual TransactionOutput
type CoinInfo struct {
	ReferenceTransactionHash string
	OutputIndex              uint32
	TransactionOutput        *block.TransactionOutput
}

// Wallet handles keeping track of the owner's coins
//
// CoinCollection is the owner of this wallet's set of coins
//
// UnseenSpentCoins is a mapping of transaction hashes (which are strings)
// to a slice of coinInfos. It's used for keeping track of coins that we've
// used in a transaction but haven't yet seen in a block.
//
// UnconfirmedSpentCoins is a mapping of Coins to number of confirmations
// (which are integers). We can't confirm that a Coin has been spent until
// we've seen enough POW on top the block containing our sent transaction.
//
// UnconfirmedReceivedCoins is a mapping of CoinInfos to number of confirmations
// (which are integers). We can't confirm we've received a Coin until
// we've seen enough POW on top the block containing our received transaction.
type Wallet struct {
	Config              *Config
	Id                  id.ID
	TransactionRequests chan *block.Transaction
	Address             string
	Balance             uint32

	// All coins
	CoinCollection map[*block.TransactionOutput]*CoinInfo

	// Not yet seen
	UnseenSpentCoins map[string][]*CoinInfo // map from string to slice of pointers 

	// Seen but not confirmed
	UnconfirmedSpentCoins    map[*CoinInfo]uint32
	UnconfirmedReceivedCoins map[*CoinInfo]uint32
}

// SetAddress sets the address
// of the node in the wallet.
func (w *Wallet) SetAddress(a string) {
	w.Address = a
}

// New creates a wallet object
func New(config *Config, id id.ID) *Wallet {
	if !config.HasWallet {
		return nil
	}
	return &Wallet{
		Config:                   config,
		Id:                       id,
		TransactionRequests:      make(chan *block.Transaction),
		Balance:                  0,
		CoinCollection:           make(map[*block.TransactionOutput]*CoinInfo),
		UnseenSpentCoins:         make(map[string][]*CoinInfo),
		UnconfirmedSpentCoins:    make(map[*CoinInfo]uint32),
		UnconfirmedReceivedCoins: make(map[*CoinInfo]uint32),
	}
}

// generateTransactionInputs creates the transaction inputs required to make a transaction.
// In addition to the inputs, it returns the amount of change the wallet holder should
// return to themselves, and the coinInfos used
func (w *Wallet) generateTransactionInputs(amount uint32, fee uint32) (uint32, []*block.TransactionInput, []*CoinInfo) {
	//TODO: optional, but we recommend using a helper like this
	total := amount + fee
	input := uint32(0)

	var ci_slice []*CoinInfo
	for _, info := range w.CoinCollection{
		_, in_bool := w.UnseenSpentCoins[info.ReferenceTransactionHash]
		if in_bool{
			continue
		}else{
			if input >= total{
				break
			}else{
				ci_slice = append(ci_slice, info)
				input = input + info.TransactionOutput.Amount
			}
		}
	}

	if input < total{
		return 0, nil, nil // the wallet doesn't have enough funds 
	}

	diff := input - total

	var all_inputs []*block.TransactionInput
	for _, info := range ci_slice{
		s,_ := info.TransactionOutput.MakeSignature(w.Id)
		trans_input := &block.TransactionInput{
			ReferenceTransactionHash: info.ReferenceTransactionHash,
			OutputIndex: info.OutputIndex,
			UnlockingScript: s,
		}
		all_inputs = append(all_inputs, trans_input)
	}


	return diff, all_inputs, ci_slice
	
}

// generateTransactionOutputs generates the transaction outputs required to create a transaction.
func (w *Wallet) generateTransactionOutputs(
	amount uint32,
	receiverPK []byte,
	change uint32,
) []*block.TransactionOutput {
	//TODO: optional, but we recommend using a helper like this
	trans_out := &block.TransactionOutput{
		Amount: amount,
		LockingScript: string(receiverPK),
	}

	all_out := []*block.TransactionOutput{trans_out}
	if change > 0 {
		new_out := &block.TransactionOutput{
			Amount: change,
			LockingScript: w.Id.GetPublicKeyString(),
		}
		all_out = append(all_out, new_out)
	}
	return all_out
}

// RequestTransaction allows the wallet to send a transaction to the node,
// which will propagate the transaction along the P2P network.
func (w *Wallet) RequestTransaction(amount uint32, fee uint32, recipientPK []byte) *block.Transaction {
	//TODO
	diff, all_inputs, ci_slice := w.generateTransactionInputs(amount, fee)

	if all_inputs != nil{
		all_out := w.generateTransactionOutputs(amount, recipientPK, diff)

		tx := &block.Transaction{
			Version: w.Config.TransactionVersion,
			Inputs: all_inputs,
			Outputs: all_out,
			LockTime: w.Config.DefaultLockTime,
		}

		for _, info := range ci_slice{
			delete(w.CoinCollection, info.TransactionOutput) // delete mapping 
			tx_hash := tx.Hash()
			w.UnseenSpentCoins[tx_hash] = append(w.UnseenSpentCoins[tx_hash], info) // append CoinInfos together 
			if w.Balance < info.TransactionOutput.Amount{
				w.Balance = 0
			}else{
				w.Balance -= info.TransactionOutput.Amount // update balance 
			}
		}

		
		// w.TransactionRequests <- tx // send a value on a channel
		go func(){ // goroutine, help to solve timeout issue 
			w.TransactionRequests <- tx
		}()

		return tx
	}
	return nil 
}

// HandleBlock handles the transactions of a new block. It:
// (1) sees if any of the inputs are ones that we've spent
// (2) sees if any of the incoming outputs on the block are ours
// (3) updates our unconfirmed coins, since we've just gotten
// another confirmation!
func (w *Wallet) HandleBlock(txs []*block.Transaction) {
	//TODO
	// (1) sees if any of the inputs are ones that we've spent
	for _, tx := range txs {
		for _, input := range tx.Inputs {
			info, in_bool := w.UnseenSpentCoins[input.ReferenceTransactionHash] 
			// map from string to slice of pointers *CoinInfo
			if in_bool{ 
				for _, coin_info := range info{
					w.UnconfirmedSpentCoins[coin_info] = 1
				}
				delete(w.UnseenSpentCoins, input.ReferenceTransactionHash)
				// delete key-value pair of a map 
			}
		}

		// (2) sees if any of the incoming outputs on the block are ours
		for idx, output := range tx.Outputs{
			if output.LockingScript == w.Id.GetPublicKeyString(){
				coin_info := &CoinInfo{
					ReferenceTransactionHash: tx.Hash(),
					OutputIndex: uint32(idx),           
					TransactionOutput: output,   
				}
				w.UnconfirmedReceivedCoins[coin_info] = 1
			}
		}
	}

	safe_amount := w.Config.SafeBlockAmount 
	for ci, count := range w.UnconfirmedSpentCoins{
		w.UnconfirmedSpentCoins[ci] = count + 1
		if count+1 >= safe_amount{
			delete(w.CoinCollection, ci.TransactionOutput) // delete mapping of CoinCollection 
			if w.Balance - ci.TransactionOutput.Amount < 0 {
				w.Balance = 0
			}else{
				w.Balance = w.Balance - ci.TransactionOutput.Amount
			}
			delete(w.UnconfirmedSpentCoins, ci)
		}
	}

	for ci, count := range w.UnconfirmedReceivedCoins{
		w.UnconfirmedReceivedCoins[ci] = count + 1
		if count+1 >= safe_amount{
			w.CoinCollection[ci.TransactionOutput] = ci
			w.Balance = w.Balance + ci.TransactionOutput.Amount
			delete(w.UnconfirmedReceivedCoins, ci)
		}
	}


}

// HandleFork handles a fork, updating the wallet's relevant fields.
func (w *Wallet) HandleFork(blocks []*block.Block, undoBlocks []*chainwriter.UndoBlock) {
	//TODO: for extra credit!
}
