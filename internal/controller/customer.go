package controller

import (
	"github.com/sgostarter/i/commerr"
	"github.com/zservicer/protorepo/gens/talkpb"
	"github.com/zservicer/talkbe/internal/defs"
)

func NewCustomer(actID, bizID string, uniqueID uint64, talkID string, createTalkFlag bool, userID uint64, chSendMessage chan *talkpb.TalkResponse) defs.Customer {
	return &customerImpl{
		actID:          actID,
		bizID:          bizID,
		uniqueID:       uniqueID,
		talkID:         talkID,
		createTalkFlag: createTalkFlag,
		userID:         userID,
		chSendMessage:  chSendMessage,
	}
}

type customerImpl struct {
	actID string
	bizID string

	uniqueID       uint64
	talkID         string
	createTalkFlag bool
	userID         uint64
	chSendMessage  chan *talkpb.TalkResponse
}

func (impl *customerImpl) GetActID() string {
	return impl.actID
}

func (impl *customerImpl) GetBizID() string {
	return impl.bizID
}

func (impl *customerImpl) CreateTalkFlag() bool {
	return impl.createTalkFlag
}

func (impl *customerImpl) GetUniqueID() uint64 {
	return impl.uniqueID
}

func (impl *customerImpl) GetTalkID() string {
	return impl.talkID
}

func (impl *customerImpl) GetUserID() uint64 {
	return impl.userID
}

func (impl *customerImpl) SendMessage(msg *talkpb.TalkResponse) error {
	select {
	case impl.chSendMessage <- msg:
	default:
		return commerr.ErrAborted
	}

	return nil
}

func (impl *customerImpl) Remove(msg string) {
	_ = impl.SendMessage(&talkpb.TalkResponse{
		Talk: &talkpb.TalkResponse_KickOut{
			KickOut: &talkpb.TalkKickOutMessage{
				Code:    -1,
				Message: msg,
			},
		},
	})
}
