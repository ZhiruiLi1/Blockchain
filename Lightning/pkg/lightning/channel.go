package lightning

import (
	"Coin/pkg/block"
	"Coin/pkg/id"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"Coin/pkg/script"
)

// Channel is our node's view of a channel
// Funder is whether we are the channel's funder
// FundingTransaction is the channel's funding transaction
// CounterPartyPubKey is the other node's public key
// State is the current state that we are at. On instantiation,
// the refund transaction is the transaction for state 0
// Transactions is the slice of transactions, indexed by state
// MyRevocationKeys is a mapping of my private revocation keys
// TheirRevocationKeys is a mapping of their private revocation keys
type Channel struct {
	Funder             bool
	FundingTransaction *block.Transaction
	State              int
	CounterPartyPubKey []byte

	MyTransactions    []*block.Transaction
	TheirTransactions []*block.Transaction

	MyRevocationKeys    map[string][]byte
	TheirRevocationKeys map[string]*RevocationInfo
}

type RevocationInfo struct {
	RevKey            []byte
	TransactionOutput *block.TransactionOutput
	OutputIndex       uint32
	TransactionHash   string
	ScriptType        int
}

// GenerateRevocationKey returns a new public, private key pair
func GenerateRevocationKey() ([]byte, []byte) {
	i, _ := id.CreateSimpleID()
	return i.GetPublicKeyBytes(), i.GetPrivateKeyBytes()
}

// CreateChannel creates a channel with another lightning node
// fee must be enough to cover two transactions! You will get back change from first
func (ln *LightningNode) CreateChannel(peer *peer.Peer, theirPubKey []byte, amount uint32, fee uint32) {
	// TODO
	cha := &Channel{
		Funder: true,
		FundingTransaction: nil,
		State: 0,
		CounterPartyPubKey: theirPubKey,
	
		MyTransactions: []*block.Transaction{},
		TheirTransactions: []*block.Transaction{},
	
		MyRevocationKeys: make(map[string][]byte), // create a new map, the key is a string and the value is []byte 
		TheirRevocationKeys: make(map[string]*RevocationInfo),
	}
	ln.Channels[peer] = cha

	// GetTransactionFromWallet     chan WalletRequest
	// WalletRequest doesn't have * so we don't need to use &
	req := WalletRequest{
		Amount: amount,
		Fee: 2 * fee,
		CounterPartyPubKey: theirPubKey,
	}
	ln.GetTransactionFromWallet <- req // <-: used for sending and receiving values through channels in Go

	// receiving a value from the ln.ReceiveTransactionFromWallet channel
	receive_trans := <- ln.ReceiveTransactionFromWallet
	public_key, private_key := GenerateRevocationKey()

	refund_trans := ln.generateRefundTransaction(theirPubKey, receive_trans, fee, public_key)

	cha.MyRevocationKeys[refund_trans.Hash()] = private_key

	open_cha := &pro.OpenChannelRequest{
		Address: ln.Address,
		PublicKey: ln.Id.GetPublicKeyBytes(),
		FundingTransaction: block.EncodeTransaction(receive_trans),
		RefundTransaction: block.EncodeTransaction(refund_trans),
	}

	res, _ := peer.Addr.OpenChannelRPC(open_cha) // peer is a struct 

	cha.FundingTransaction = block.DecodeTransaction(res.SignedFundingTransaction)
	trans1 := block.DecodeTransaction(res.SignedRefundTransaction)
	tmp1 := []*block.Transaction{trans1}
	cha.MyTransactions = append(tmp1, cha.MyTransactions...) // ...:  passing its elements as separate arguments

	tmp2 := []*block.Transaction{trans1}
	cha.TheirTransactions = append(tmp2, cha.TheirTransactions...)

	ln.ValidateAndSign(receive_trans)
	ln.BroadcastTransaction <- receive_trans

}

// UpdateState is called to update the state of a channel.
func (ln *LightningNode) UpdateState(peer *peer.Peer, tx *block.Transaction) {
	// TODO
	cha := ln.Channels[peer]
	req := &pro.TransactionWithAddress{
		Address: ln.Address,
		Transaction: block.EncodeTransaction(tx),
	}
	updated_tx, _ := peer.Addr.GetUpdatedTransactionsRPC(req)

	trans1 := block.DecodeTransaction(updated_tx.GetSignedTransaction())
	cha.MyTransactions = append(cha.MyTransactions, trans1)

	trans2 := block.DecodeTransaction(updated_tx.GetUnsignedTransaction())
	ln.ValidateAndSign(trans2)

	cha.TheirTransactions = append(cha.TheirTransactions, trans2)


	trans3 := cha.MyTransactions[cha.State].Hash()
	req_key := &pro.SignedTransactionWithKey{
		Address: ln.Address,
		SignedTransaction: updated_tx.SignedTransaction,
		RevocationKey: cha.MyRevocationKeys[trans3],
	}
	revo_key, _ := peer.Addr.GetRevocationKeyRPC(req_key)
	
	cha.State ++

	ind := uint32(0)
	if cha.Funder {
		ind = 1
	}

	new_script := updated_tx.GetSignedTransaction().Outputs[ind].LockingScript
	script_type, _ := script.DetermineScriptType(new_script)

	trans_out := block.DecodeTransaction(updated_tx.GetSignedTransaction()).Outputs[ind]
	trans_hash := block.DecodeTransaction(updated_tx.GetSignedTransaction()).Hash()
	revo := &RevocationInfo{
		RevKey: revo_key.Key,
		TransactionOutput: trans_out,
		OutputIndex: ind,
		TransactionHash: trans_hash,
		ScriptType: script_type,
	}

	cha.TheirRevocationKeys[trans_hash] = revo
}
