package impls

import (
	"context"
	"strconv"
	"sync"

	"github.com/godruoyi/go-snowflake"
	"github.com/sbasestarter/bizinters/talkinters"
	"github.com/sgostarter/i/commerr"
	"golang.org/x/exp/slices"
)

func NewMemModel() talkinters.Model {
	return &memModelImpl{
		talks: make(map[string]*memTalk),
	}
}

type memTalk struct {
	info     talkinters.TalkInfoR
	messages []talkinters.TalkMessageR
}

type memModelImpl struct {
	talksLock sync.Mutex
	talks     map[string]*memTalk
}

func (impl *memModelImpl) CreateTalk(ctx context.Context, talkInfo *talkinters.TalkInfoW) (talkID string, err error) {
	impl.talksLock.Lock()
	defer impl.talksLock.Unlock()

	talkID = strconv.FormatUint(snowflake.ID(), 10)

	impl.talks[talkID] = &memTalk{
		info: talkinters.TalkInfoR{
			TalkID:    talkID,
			TalkInfoW: *talkInfo,
		},
		messages: make([]talkinters.TalkMessageR, 0, 10),
	}

	return
}

func (impl *memModelImpl) OpenTalk(ctx context.Context, actIDs, bizIDs []string, talkID string) error {
	impl.talksLock.Lock()
	defer impl.talksLock.Unlock()

	talk, ok := impl.talks[talkID]
	if !ok {
		return commerr.ErrNotFound
	}

	if len(actIDs) > 0 && !slices.Contains(actIDs, talk.info.ActID) {
		return commerr.ErrNotFound
	}

	if len(bizIDs) > 0 && !slices.Contains(bizIDs, talk.info.BizID) {
		return commerr.ErrNotFound
	}

	talk.info.Status = talkinters.TalkStatusOpened

	return nil
}

func (impl *memModelImpl) CloseTalk(ctx context.Context, actIDs, bizIDs []string, talkID string) error {
	impl.talksLock.Lock()
	defer impl.talksLock.Unlock()

	talk, ok := impl.talks[talkID]
	if !ok {
		return commerr.ErrNotFound
	}

	if len(actIDs) > 0 && !slices.Contains(actIDs, talk.info.ActID) {
		return commerr.ErrNotFound
	}

	if len(bizIDs) > 0 && !slices.Contains(bizIDs, talk.info.BizID) {
		return commerr.ErrNotFound
	}

	talk.info.Status = talkinters.TalkStatusClosed

	return nil
}

func (impl *memModelImpl) AddTalkMessage(ctx context.Context, talkID string, message *talkinters.TalkMessageW) error {
	impl.talksLock.Lock()
	defer impl.talksLock.Unlock()

	talk, ok := impl.talks[talkID]
	if !ok {
		return commerr.ErrNotFound
	}

	talk.messages = append(talk.messages, talkinters.TalkMessageR{
		MessageID:    strconv.FormatUint(snowflake.ID(), 10),
		TalkMessageW: *message,
	})

	return nil
}

func (impl *memModelImpl) GetTalkMessages(ctx context.Context, talkID string, offset, count int64) (messages []*talkinters.TalkMessageR, err error) {
	impl.talksLock.Lock()
	defer impl.talksLock.Unlock()

	talk, ok := impl.talks[talkID]
	if !ok {
		err = commerr.ErrNotFound

		return
	}

	messages = make([]*talkinters.TalkMessageR, 0, len(talk.messages))

	for _, message := range talk.messages {
		messages = append(messages, &talkinters.TalkMessageR{
			MessageID:    message.MessageID,
			TalkMessageW: message.TalkMessageW,
		})
	}

	return
}

func (impl *memModelImpl) QueryTalks(ctx context.Context, actIDs, bizIDs []string, creatorID, serviceID uint64, talkID string, statuses []talkinters.TalkStatus) (talks []*talkinters.TalkInfoR, err error) {
	impl.talksLock.Lock()
	defer impl.talksLock.Unlock()

	talks = make([]*talkinters.TalkInfoR, 0, 10)

	for id, talk := range impl.talks {
		if len(actIDs) > 0 && !slices.Contains(actIDs, talk.info.ActID) {
			continue
		}

		if len(bizIDs) > 0 && !slices.Contains(bizIDs, talk.info.BizID) {
			continue
		}

		if creatorID > 0 {
			if talk.info.CreatorID != creatorID {
				continue
			}
		}

		if serviceID > 0 {
			if talk.info.ServiceID != serviceID {
				continue
			}
		}

		if talkID != "" {
			if id != talkID {
				continue
			}
		}

		if len(statuses) > 0 {
			var statusOk bool

			for _, status := range statuses {
				if talk.info.Status == status {
					statusOk = true

					break
				}
			}

			if !statusOk {
				continue
			}
		}

		talks = append(talks, &talkinters.TalkInfoR{
			TalkID:    id,
			TalkInfoW: talk.info.TalkInfoW,
		})
	}

	return
}

func (impl *memModelImpl) GetPendingTalkInfos(ctx context.Context, actIDs, bizIDs []string) (talks []*talkinters.TalkInfoR, err error) {
	impl.talksLock.Lock()
	defer impl.talksLock.Unlock()

	talks = make([]*talkinters.TalkInfoR, 0, 10)

	for talkID, talk := range impl.talks {
		if talk.info.ServiceID > 0 {
			continue
		}

		if len(actIDs) > 0 && !slices.Contains(actIDs, talk.info.ActID) {
			continue
		}

		if len(bizIDs) > 0 && !slices.Contains(bizIDs, talk.info.BizID) {
			continue
		}

		talks = append(talks, &talkinters.TalkInfoR{
			TalkID:    talkID,
			TalkInfoW: talk.info.TalkInfoW,
		})
	}

	return
}

func (impl *memModelImpl) UpdateTalkServiceID(ctx context.Context, actIDs, bizIDs []string, talkID string, serviceID uint64) (err error) {
	impl.talksLock.Lock()
	defer impl.talksLock.Unlock()

	talk, ok := impl.talks[talkID]
	if !ok {
		err = commerr.ErrNotFound

		return
	}

	if len(actIDs) > 0 && !slices.Contains(actIDs, talk.info.ActID) {
		return commerr.ErrNotFound
	}

	if len(bizIDs) > 0 && !slices.Contains(bizIDs, talk.info.BizID) {
		return commerr.ErrNotFound
	}

	talk.info.ServiceID = serviceID

	return
}
