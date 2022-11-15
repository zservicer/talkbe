package defs

import "github.com/zservicer/protorepo/gens/talkpb"

type Servicer interface {
	GetUserID() uint64
	GetUniqueID() uint64
	SendMessage(msg *talkpb.ServiceResponse) error
	Remove(msg string)
}
