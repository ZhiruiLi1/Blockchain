package miner

import (
	"Coin/pkg/block"
	"context"
	"fmt"
	"math"
	"time"
	"bytes"
)

// Mine When asked to mine, the miner selects the transactions
// with the highest priority to add to the mining pool.
func (m *Miner) Mine() *block.Block { // this is a Block instance from the block package 
	//TODO
	// check if there are enough transactions worth to mine 
	if m.TxPool.PriorityMet() == false{
		return nil 
	}

	// set mining to true 
	m.Mining.Store(true)

	// select transactions to mine 
	txs := m.NewMiningPool()

	// construct blocks
	coinbase_txs := m.GenerateCoinbaseTransaction(txs)
	all_txs := []*block.Transaction{coinbase_txs}

	for _, tx := range txs{
		all_txs = append(all_txs, tx)
	}

	mr := block.CalculateMerkleRoot(all_txs)

	// Block struct needs *Header and []*Transaction
	new_block := &block.Block{
		Header: &block.Header{
			Version: 0,
			PreviousHash: m.PreviousHash,
			MerkleRoot: mr, 
			DifficultyTarget: string(m.DifficultyTarget),
			Nonce: 0, 
			Timestamp: uint32(time.Now().Unix()), // using go 'time' package 
		}, 
		Transactions: all_txs,
	}

	context, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	// The context value is the new context derived from the parent context, with the timeout applied
	// The cancel function is a function that you can call to cancel the context prematurely (before the timeout)
	defer cancel()

	// find 0s
	nonce_bool := m.CalculateNonce(context, new_block)

	if nonce_bool { // if successfully find the nonce 
		m.Mining.Store(false)
		m.SendBlock <- new_block
		m.HandleBlock(new_block)
		return new_block
	}

	return nil
}

// CalculateNonce finds a winning nonce for a block. It uses context to
// know whether it should quit before it finds a nonce (if another block
// was found). ASICSs are optimized for this task.
func (m *Miner) CalculateNonce(ctx context.Context, b *block.Block) bool {
	nonce := uint32(0)

	for {
		select {
		case <-ctx.Done():
			return false
		default:
			if nonce < math.MaxUint32 {
				b.Header.Nonce = nonce
				hash := []byte(b.Hash())

				if bytes.Compare(hash, m.DifficultyTarget) == -1 {
					return true
				}

				nonce++
			} else {
				return false
			}
		}
	}
}

// GenerateCoinbaseTransaction generates a coinbase
// transaction based off the transactions in the mining pool.
// It does this by combining the fee reward to the minting reward,
// and sending that sum to itself.
func (m *Miner) GenerateCoinbaseTransaction(txs []*block.Transaction) *block.Transaction {
	count := uint32(0)
	sums, _ := m.getInputSums(txs)
	rewards := m.CalculateMintingReward()
	for _, x := range sums{  // sum of the inputs 
		count += x
	}
	for _, t := range txs{ // minus the sum of the outputs 
		for _, out := range t.Outputs{
			count -= out.Amount
		}
	}

	total_count := rewards + count 
	checking := m.Id.GetPublicKeyString()

	return &block.Transaction{
		Version: 0,
		Inputs: []*block.TransactionInput{},
		Outputs: []*block.TransactionOutput{&block.TransactionOutput{Amount: total_count, LockingScript: checking}},
		// The Outputs field contains a list (slice) of pointers to block.TransactionOutput structs.
		LockTime: m.Config.DefineLockTime,
	}

}

// getInputSums returns the sums of the inputs of a slice of transactions,
// as well as an error if the function fails. This function sends a request to
// its GetInputsSum channel, which the node picks up. The node then handles
// the request, returning the sum of the inputs in the InputsSum channel.
// This function times out after 1 second.
func (m *Miner) getInputSums(txs []*block.Transaction) ([]uint32, error) {
	// time out after 1 second
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	// ask the node to sum the inputs for our transactions
	m.GetInputSums <- txs
	// wait until we get a response from the node in our SumInputs channel
	for {
		select {
		case <-ctx.Done():
			// Oops! We ran out of time
			return []uint32{0}, fmt.Errorf("[miner.sumInputs] Error: timed out")
		case sums := <-m.InputSums:
			// Yay! We got a response from our node.
			return sums, nil
		}
	}
}

// CalculateMintingReward calculates
// the minting reward the miner should receive based
// on the current chain length.
func (m *Miner) CalculateMintingReward() uint32 {
	c := m.Config
	chainLength := m.ChainLength.Load()
	if chainLength >= c.SubsidyHalvingRate*c.MaxHalvings {
		return 0
	}
	halvings := chainLength / c.SubsidyHalvingRate
	rwd := c.InitialSubsidy
	rwd /= uint32(math.Pow(2, float64(halvings)))
	return rwd
}
