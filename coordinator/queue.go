package main

import (
	"log"

	"github.com/rsbondi/multifund/funder"
	"github.com/rsbondi/multifund/wallet"
)

type JoinQueue struct {
	Participants map[int]funder.FundingInfo
	Tx           *wallet.Transaction
	pid          *int // participant
	sid          *int // signature
	cid          map[int]bool
}

func NewJoinQueue() JoinQueue {
	log.Println("I SHOULD ONLY EVER DO THIS ONCE!!!!!!!!")
	parties := make(map[int]funder.FundingInfo)
	p := 0
	s := 1
	cid := make(map[int]bool, N_PARTICIPANTS)
	return JoinQueue{
		Participants: parties,
		Tx:           &wallet.Transaction{},
		pid:          &p,
		sid:          &s,
		cid:          cid,
	}
}

func (j JoinQueue) Add(f funder.FundingInfo) int {
	*j.pid++
	j.Participants[*j.pid] = f
	return *j.pid
}

func (j JoinQueue) SetTx(tx wallet.Transaction) {
	log.Printf("SetTx: %x", tx.Unsigned)
	*j.Tx = tx
}
