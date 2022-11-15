package defs

import (
	"context"

	"github.com/sbasestarter/bizinters/talkinters"
)

type ModelEx interface {
	talkinters.Model

	TalkExists(ctx context.Context, talkID string) (bool, error)
	GetTalkInfo(ctx context.Context, talkID string) (*talkinters.TalkInfoR, error)
	GetServicerTalkInfos(ctx context.Context, servicerID uint64) ([]*talkinters.TalkInfoR, error)
	GetTalkServicerID(ctx context.Context, talkID string) (servicerID uint64, err error)
}
