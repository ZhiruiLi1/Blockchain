package lightning

import (
	"Coin/pkg/address"
	"Coin/pkg/peer"
	"Coin/pkg/pro"
	"context"
	"time"
	"fmt"
	"Coin/pkg/block"
	"Coin/pkg/utils"
	"Coin/pkg/script"
)

// Version was copied directly from pkg/server.go. Only changed the function receiver and types
func (ln *LightningNode) Version(ctx context.Context, in *pro.VersionRequest) (*pro.Empty, error) {
	// Reject all outdated versions (this is not true to Satoshi Client)
	if in.Version != ln.Config.Version {
		return &pro.Empty{}, nil
	}
	// If addr map is full or does not contain addr of ver, reject
	newAddr := address.New(in.AddrMe, uint32(time.Now().UnixNano()))
	if ln.AddressDB.Get(newAddr.Addr) != nil {
		err := ln.AddressDB.UpdateLastSeen(newAddr.Addr, newAddr.LastSeen)
		if err != nil {
			return &pro.Empty{}, nil
		}
	} else if err := ln.AddressDB.Add(newAddr); err != nil {
		return &pro.Empty{}, nil
	}
	newPeer := peer.New(ln.AddressDB.Get(newAddr.Addr), in.Version, in.BestHeight)
	// Check if we are waiting for a ver in response to a ver, do not respond if this is a confirmation of peering
	pendingVer := newPeer.Addr.SentVer != time.Time{} && newPeer.Addr.SentVer.Add(ln.Config.VersionTimeout).After(time.Now())
	if ln.PeerDb.Add(newPeer) && !pendingVer {
		newPeer.Addr.SentVer = time.Now()
		_, err := newAddr.VersionRPC(&pro.VersionRequest{
			Version:    ln.Config.Version,
			AddrYou:    in.AddrYou,
			AddrMe:     ln.Address,
			BestHeight: ln.BlockHeight,
		})
		if err != nil {
			return &pro.Empty{}, err
		}
	}
	return &pro.Empty{}, nil
}

// OpenChannel is called by another lightning node that wants to open a channel with us
func (ln *LightningNode) OpenChannel(ctx context.Context, in *pro.OpenChannelRequest) (*pro.OpenChannelResponse, error) {
	//TODO
	
	all_addresses := in.GetAddress()
	p := ln.PeerDb.Get(all_addresses)

	if p == nil {
		return nil, fmt.Errorf("the peer is unknown!")
	}

	_, ok := ln.Channels[p]
	if ok {
		return nil, fmt.Errorf("the channel is already existed!")
	}

	tx_f := in.GetFundingTransaction()
	tx_r := in.GetRefundTransaction()

	tx_f_decode := block.DecodeTransaction(tx_f)
	tx_r_decode := block.DecodeTransaction(tx_r)

	ok1 := ln.ValidateAndSign(tx_f_decode)
	if ok1 != nil {
		return nil, ok1
	}

	ok2 := ln.ValidateAndSign(tx_r_decode)
	if ok2 != nil {
		return nil, ok2
	}

	cha := &Channel{
		Funder: false,
		FundingTransaction: tx_f_decode,
		State: 0,
		CounterPartyPubKey: in.GetPublicKey(),
	
		MyTransactions: []*block.Transaction{tx_r_decode},
		TheirTransactions: []*block.Transaction{tx_r_decode},
	
		MyRevocationKeys: make(map[string][]byte), // create a new map, the key is a string and the value is []byte 
		TheirRevocationKeys: make(map[string]*RevocationInfo),
	}

	ln.Channels[p] = cha

	_, re_key := GenerateRevocationKey()
	// Channels    map[*peer.Peer]*Channel
	// MyRevocationKeys    map[string][]byte
	ln.Channels[p].MyRevocationKeys[tx_r_decode.Hash()] = re_key

	cha_response := &pro.OpenChannelResponse{
		PublicKey: ln.Id.GetPublicKeyBytes(),
		SignedFundingTransaction: block.EncodeTransaction(tx_f_decode),
		SignedRefundTransaction: block.EncodeTransaction(tx_r_decode),
	}


	return cha_response, nil 
}

func (ln *LightningNode) GetUpdatedTransactions(ctx context.Context, in *pro.TransactionWithAddress) (*pro.UpdatedTransactions, error) {
	// TODO

	p := ln.PeerDb.Get(in.Address) // get peers 
	if p == nil{
		return nil, fmt.Errorf("the peer is unknown!")
	}

	tx := block.DecodeTransaction(in.Transaction)
	hashTx := tx.Hash()

	s, ok := utils.Sign(ln.Id.GetPrivateKey(), []byte(hashTx))
	// []byte{}: an empty byte slice
	// []byte(hashTx): converts the variable hashTx into bytes slice 

	if ok != nil{
		return nil, ok
	}

	in.Transaction.Witnesses = append(in.Transaction.Witnesses, s)

	public_key_bytes, private_key_bytes := GenerateRevocationKey()

	trans := ln.generateTransactionWithCorrectScripts(p, block.DecodeTransaction(in.Transaction), public_key_bytes)

	cha := ln.Channels[p]
	cha.TheirTransactions = append(cha.TheirTransactions, trans)
	cha.MyRevocationKeys[hashTx] = private_key_bytes

	new_trans := &pro.UpdatedTransactions{
		SignedTransaction: in.Transaction,
		UnsignedTransaction: block.EncodeTransaction(trans),
	}

	return new_trans, nil
}

func (ln *LightningNode) GetRevocationKey(ctx context.Context, in *pro.SignedTransactionWithKey) (*pro.RevocationKey, error) {
	// TODO
	p := ln.PeerDb.Get(in.Address)
	if p == nil {
		return nil, fmt.Errorf("the peer is unknown!")
	}

	cha := ln.Channels[p]
	de_trans := block.DecodeTransaction(in.GetSignedTransaction())
	cha.MyTransactions = append(cha.MyTransactions, de_trans)

	ind := uint32(1)
	if ! cha.Funder{
		ind = 0
	}

	signed_trans := in.GetSignedTransaction()
	output := signed_trans.Outputs[ind]

	script_t, ok := script.DetermineScriptType(output.LockingScript)
	if ok != nil {
		return nil, ok 
	}

	de_trans2 := block.DecodeTransaction(in.GetSignedTransaction())
	revo := &RevocationInfo{
		RevKey: in.GetRevocationKey(),
		TransactionOutput: de_trans2.Outputs[ind],
		OutputIndex: ind,
		TransactionHash: de_trans2.Hash(),
		ScriptType: script_t,
	}
	cha.TheirRevocationKeys[de_trans2.Hash()] = revo

	revo_key := cha.MyRevocationKeys[de_trans2.Hash()]

	cha.State ++ 

	revo_fin := &pro.RevocationKey{
		Key: revo_key,
	}

	return revo_fin, nil
}
