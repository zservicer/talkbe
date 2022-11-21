package defs

import "github.com/zservicer/protorepo/gens/talkpb"

type Customer interface {
	GetActID() string
	GetBizID() string

	GetUniqueID() uint64
	GetTalkID() string
	GetUserID() uint64
	SendMessage(msg *talkpb.TalkResponse) error
	Remove(msg string)

	CreateTalkFlag() bool
}
