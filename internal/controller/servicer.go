package controller

import (
	"github.com/sgostarter/i/commerr"
	"github.com/zservicer/protorepo/gens/talkpb"
	"github.com/zservicer/talkbe/internal/defs"
)

func NewServicer(userID, uniqueID uint64, chSendMessage chan *talkpb.ServiceResponse, actIDs, bizIDs []string) defs.Servicer {
	return &servicerImpl{
		userID:        userID,
		uniqueID:      uniqueID,
		chSendMessage: chSendMessage,
		actIDs:        actIDs,
		bizIDs:        bizIDs,
	}
}

type servicerImpl struct {
	userID        uint64
	uniqueID      uint64
	chSendMessage chan *talkpb.ServiceResponse

	actIDs []string
	bizIDs []string
}

func (impl *servicerImpl) GetUserID() uint64 {
	return impl.userID
}

func (impl *servicerImpl) GetUniqueID() uint64 {
	return impl.uniqueID
}

func (impl *servicerImpl) SendMessage(msg *talkpb.ServiceResponse) error {
	select {
	case impl.chSendMessage <- msg:
	default:
		return commerr.ErrAborted
	}

	return nil
}

func (impl *servicerImpl) Remove(msg string) {
	_ = impl.SendMessage(&talkpb.ServiceResponse{
		Response: &talkpb.ServiceResponse_KickOut{
			KickOut: &talkpb.TalkKickOutMessage{
				Code:    -1,
				Message: msg,
			},
		},
	})
}

func (impl *servicerImpl) GetActIDs() []string {
	return impl.actIDs
}

func (impl *servicerImpl) GetBizIDs() []string {
	return impl.bizIDs
}
