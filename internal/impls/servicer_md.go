package impls

import (
	"context"

	"github.com/sbasestarter/bizinters/talkinters"
	"github.com/sgostarter/i/l"
	"github.com/zservicer/protorepo/gens/talkpb"
	"github.com/zservicer/talkbe/internal/defs"
	"github.com/zservicer/talkbe/internal/vo"
	"golang.org/x/exp/slices"
)

func NewServicerMD(mdi defs.ServicerMDI, logger l.Wrapper) defs.ServicerMD {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	impl := &servicerMDImpl{
		mdi:       mdi,
		logger:    logger,
		servicers: make(map[uint64]map[uint64]defs.Servicer),
	}

	mdi.SetServicerObserver(impl)

	return impl
}

type servicerMDImpl struct {
	mrRunner defs.MainRoutineRunner
	mdi      defs.ServicerMDI
	logger   l.Wrapper

	servicers map[uint64]map[uint64]defs.Servicer // servicerID - servicerN - servicer
}

//
// defs.ServicerObserver
//

func (impl *servicerMDImpl) OnMessageIncoming(senderUniqueID uint64, talkID string, message *talkinters.TalkMessageW) {
	impl.mrRunner.Post(func() {
		impl.sendResponseToServicersForTalk(0, talkID, &talkpb.ServiceResponse{
			Response: &talkpb.ServiceResponse_Message{
				Message: &talkpb.ServiceTalkMessageResponse{
					TalkId:  talkID,
					Message: vo.TalkMessageDB2Pb4Servicer(message),
				},
			},
		})
	})
}

func (impl *servicerMDImpl) OnTalkCreate(talkID string) {
	impl.mrRunner.Post(func() {
		talkInfo, err := impl.mdi.GetM().GetTalkInfo(context.TODO(), nil, nil, talkID)
		if err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkInfoFailed")

			return
		}

		resp := &talkpb.ServiceResponse{
			Response: &talkpb.ServiceResponse_Detach{ // FIXME use create message?
				Detach: &talkpb.ServiceDetachTalkResponse{
					Talk: vo.TalkInfoRDb2Pb(talkInfo),
				},
			},
		}
		impl.send4AllServicers(talkInfo.ActID, talkInfo.BizID, func(servicer defs.Servicer) error {
			return servicer.SendMessage(resp)
		})
	})
}

func (impl *servicerMDImpl) OnTalkClose(talkID string) {
	impl.mrRunner.Post(func() {
		talkInfo, err := impl.mdi.GetM().GetTalkInfo(context.TODO(), nil, nil, talkID)
		if err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkInfoFailed")

			return
		}

		resp := &talkpb.ServiceResponse{
			Response: &talkpb.ServiceResponse_Close{
				Close: &talkpb.ServiceTalkClose{
					TalkId: talkID,
				},
			},
		}

		impl.send4AllServicers(talkInfo.ActID, talkInfo.BizID, func(servicer defs.Servicer) error {
			return servicer.SendMessage(resp)
		})
	})
}

func (impl *servicerMDImpl) OnServicerAttachMessage(talkID string, servicerID uint64) {
	impl.mrRunner.Post(func() {
		talkInfo, err := impl.mdi.GetM().GetTalkInfo(context.TODO(), nil, nil, talkID)
		if err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkInfoFailed")

			return
		}

		resp := &talkpb.ServiceResponse{
			Response: &talkpb.ServiceResponse_Attach{
				Attach: &talkpb.ServiceAttachTalkResponse{
					Talk:              vo.TalkInfoRDb2Pb(talkInfo),
					AttachedServiceId: servicerID,
				},
			},
		}

		impl.send4AllServicers(talkInfo.ActID, talkInfo.BizID, func(servicer defs.Servicer) error {
			return servicer.SendMessage(resp)
		})

		talkWithMessages, err := impl.getTalkInfoWithMessages(context.TODO(), talkID)
		if err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("getTalkInfoWithMessagesFailed")

			return
		}

		resp = &talkpb.ServiceResponse{
			Response: &talkpb.ServiceResponse_Reload{
				Reload: &talkpb.ServiceTalkReloadResponse{
					Talk: talkWithMessages,
				},
			},
		}

		impl.send4AllOneServicer(servicerID, func(servicer defs.Servicer) error {
			return servicer.SendMessage(resp)
		})
	})
}

func (impl *servicerMDImpl) OnServicerDetachMessage(talkID string, servicerID uint64) {
	talkInfo, err := impl.mdi.GetM().GetTalkInfo(context.TODO(), nil, nil, talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkInfoFailed")

		return
	}

	impl.mrRunner.Post(func() {
		resp := &talkpb.ServiceResponse{
			Response: &talkpb.ServiceResponse_Detach{
				Detach: &talkpb.ServiceDetachTalkResponse{
					Talk:              vo.TalkInfoRDb2Pb(talkInfo),
					DetachedServiceId: servicerID,
				},
			},
		}
		impl.send4AllServicers(talkInfo.ActID, talkInfo.BizID, func(servicer defs.Servicer) error {
			return servicer.SendMessage(resp)
		})
	})
}

//
// defs.ServicerMD
//

func (impl *servicerMDImpl) Setup(mr defs.MainRoutineRunner) {
	impl.mrRunner = mr
}

func (impl *servicerMDImpl) InstallServicer(ctx context.Context, servicer defs.Servicer) {
	if servicer == nil {
		impl.logger.Error("noServicer")

		return
	}

	talkIDs, _ := impl.sendAttachedTalks(ctx, servicer)
	for _, talkID := range talkIDs {
		_ = impl.mdi.AddTrackTalk(ctx, talkID)
	}

	_ = impl.sendPendingTalks(ctx, servicer)

	if _, ok := impl.servicers[servicer.GetUserID()]; !ok {
		impl.servicers[servicer.GetUserID()] = make(map[uint64]defs.Servicer)
	}

	impl.servicers[servicer.GetUserID()][servicer.GetUniqueID()] = servicer
}

func (impl *servicerMDImpl) UninstallServicer(ctx context.Context, servicer defs.Servicer) {
	talkServicers, ok := impl.servicers[servicer.GetUserID()]
	if !ok {
		impl.logger.Warn("noUserService")

		return
	}

	delete(talkServicers, servicer.GetUniqueID())

	if len(talkServicers) == 0 {
		delete(impl.servicers, servicer.GetUserID())

		talkInfos, _ := impl.mdi.GetM().GetServicerTalkInfos(ctx, servicer.GetActIDs(), servicer.GetBizIDs(), servicer.GetUserID())
		for _, info := range talkInfos {
			impl.mdi.RemoveTrackTalk(context.TODO(), info.TalkID)
		}
	}
}

func (impl *servicerMDImpl) ServicerAttachTalk(ctx context.Context, talkID string, servicer defs.Servicer) {
	if talkID == "" || servicer == nil {
		impl.logger.WithFields(l.StringField("talkID", talkID)).Error("noServicerOrTalkID")

		return
	}

	servicerID, err := impl.mdi.GetM().GetTalkServicerID(ctx, servicer.GetActIDs(), servicer.GetBizIDs(), talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkServicerIDFailed")

		return
	}

	if servicerID > 0 {
		if servicerID == servicer.GetUserID() {
			impl.logger.WithFields(l.StringField("talkID", talkID),
				l.UInt64Field("userID", servicer.GetUserID())).Warn("talkAlreadyAttached")

			return
		}
	}

	err = impl.mdi.GetM().UpdateTalkServiceID(ctx, servicer.GetActIDs(), servicer.GetBizIDs(), talkID, servicer.GetUserID())
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("UpdateTalkServiceID")
	}

	impl.mdi.SendServicerAttachMessage(talkID, servicer.GetUserID())
}

func (impl *servicerMDImpl) ServicerDetachTalk(ctx context.Context, talkID string, servicer defs.Servicer) {
	if talkID == "" || servicer == nil {
		impl.logger.WithFields(l.StringField("talkID", talkID)).Error("noServicerOrTalkID")

		return
	}

	servicerID, err := impl.mdi.GetM().GetTalkServicerID(ctx, servicer.GetActIDs(), servicer.GetBizIDs(), talkID)
	if err != nil {
		impl.logger.WithFields(l.StringField("talkID", talkID)).Error("GetTalkServicerIDFailed")

		return
	}

	if servicerID != servicer.GetUserID() {
		if err = servicer.SendMessage(&talkpb.ServiceResponse{
			Response: &talkpb.ServiceResponse_Notify{
				Notify: &talkpb.ServiceTalkNotifyResponse{
					Msg: "talkNotAttached",
				},
			},
		}); err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

			return
		}

		return
	}

	if err = impl.mdi.GetM().UpdateTalkServiceID(ctx, servicer.GetActIDs(), servicer.GetBizIDs(), talkID, 0); err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("UpdateTalkServiceID")
	}

	impl.mdi.SendServiceDetachMessage(talkID, servicer.GetUserID())
}

func (impl *servicerMDImpl) ServicerQueryAttachedTalks(ctx context.Context, servicer defs.Servicer) {
	_, _ = impl.sendAttachedTalks(ctx, servicer)
}

func (impl *servicerMDImpl) ServicerQueryPendingTalks(ctx context.Context, servicer defs.Servicer) {
	_ = impl.sendPendingTalks(ctx, servicer)
}

func (impl *servicerMDImpl) ServicerReloadTalk(ctx context.Context, servicer defs.Servicer, talkID string) {
	talk, err := impl.getTalkInfoWithMessages(ctx, talkID)
	if err != nil {
		return
	}

	if err = servicer.SendMessage(&talkpb.ServiceResponse{
		Response: &talkpb.ServiceResponse_Reload{
			Reload: &talkpb.ServiceTalkReloadResponse{
				Talk: talk,
			},
		},
	}); err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")
	}
}

func (impl *servicerMDImpl) ServiceMessage(ctx context.Context, servicer defs.Servicer, talkID string,
	seqID uint64, message *talkinters.TalkMessageW) {
	if servicer == nil || talkID == "" || message == nil {
		impl.logger.Error("nilParameters")

		return
	}

	servicerID, err := impl.mdi.GetM().GetTalkServicerID(ctx, servicer.GetActIDs(), servicer.GetBizIDs(), talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkServicerIDFailed")

		return
	}

	if servicerID != servicer.GetUserID() {
		impl.logger.WithFields(l.StringField("talkID", talkID), l.UInt64Field("curServicerID", servicer.GetUserID()),
			l.UInt64Field("talkServicerID", servicerID)).Error("invalidServicerID")

		return
	}

	servicersMap := impl.servicers[servicer.GetUserID()]

	if err = servicer.SendMessage(&talkpb.ServiceResponse{
		Response: &talkpb.ServiceResponse_MessageConfirmed{
			MessageConfirmed: &talkpb.ServiceMessageConfirmed{
				SeqId: seqID,
				At:    uint64(message.At),
			},
		},
	}); err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

		servicer.Remove("SendMessageFailed")

		delete(servicersMap, servicer.GetUniqueID())
	}

	impl.mdi.SendMessage(servicer.GetUniqueID(), talkID, message)
}

//
//
//

func (impl *servicerMDImpl) sendAttachedTalks(ctx context.Context, servicer defs.Servicer) (talkIDs []string, err error) {
	talkInfos, err := impl.mdi.GetM().GetServicerTalkInfos(ctx, servicer.GetActIDs(), servicer.GetBizIDs(), servicer.GetUserID())
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetServicerTalkInfosFailed")

		return
	}

	talks := make([]*talkpb.ServiceTalkInfoAndMessages, 0, len(talkInfos))

	for _, talkInfo := range talkInfos {
		talkMessages, _ := impl.mdi.GetM().GetTalkMessages(ctx, talkInfo.TalkID, 0, 0)
		talkIDs = append(talkIDs, talkInfo.TalkID)

		talks = append(talks, &talkpb.ServiceTalkInfoAndMessages{
			TalkInfo: vo.TalkInfoRDb2Pb(talkInfo),
			Messages: vo.TalkMessagesRDb2Pb(talkMessages),
		})
	}

	err = servicer.SendMessage(&talkpb.ServiceResponse{
		Response: &talkpb.ServiceResponse_Talks{
			Talks: &talkpb.ServiceAttachedTalksResponse{
				Talks: talks,
			},
		},
	})
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")
	}

	return
}

func (impl *servicerMDImpl) sendPendingTalks(ctx context.Context, servicer defs.Servicer) (err error) {
	talkInfos, err := impl.mdi.GetM().GetPendingTalkInfos(ctx, servicer.GetActIDs(), servicer.GetBizIDs())
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetPendingTalkInfosFailed")

		return
	}

	err = servicer.SendMessage(&talkpb.ServiceResponse{
		Response: &talkpb.ServiceResponse_PendingTalks{
			PendingTalks: &talkpb.ServicePendingTalksResponse{
				Talks: vo.TalkInfoRsDB2Pb(talkInfos),
			},
		},
	})
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")
	}

	return
}

func (impl *servicerMDImpl) getTalkInfoWithMessages(ctx context.Context, talkID string) (*talkpb.ServiceTalkInfoAndMessages, error) {
	talkInfo, err := impl.mdi.GetM().GetTalkInfo(ctx, nil, nil, talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkInfoFailed")

		return nil, err
	}

	talkMessages, err := impl.mdi.GetM().GetTalkMessages(ctx, talkID, 0, 0)
	if err != nil {
		impl.logger.Error("NoTalkIDMessage")

		return nil, err
	}

	return &talkpb.ServiceTalkInfoAndMessages{
		TalkInfo: vo.TalkInfoRDb2Pb(talkInfo),
		Messages: vo.TalkMessagesRDb2Pb(talkMessages),
	}, nil
}

func (impl *servicerMDImpl) sendResponseToServicersForTalk(excludedUniqueID uint64, talkID string, resp *talkpb.ServiceResponse) {
	servicerID, err := impl.mdi.GetM().GetTalkServicerID(context.TODO(), nil, nil, talkID)
	if err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("GetTalkServicerIDFailed")

		return
	}

	if servicerID == 0 {
		return
	}

	servicersMap := impl.servicers[servicerID]

	for _, servicer := range servicersMap {
		if servicer.GetUniqueID() == excludedUniqueID {
			continue
		}

		if err = servicer.SendMessage(resp); err != nil {
			impl.logger.WithFields(l.ErrorField(err)).Error("SendMessageFailed")

			servicer.Remove("SendMessageFailed")

			delete(servicersMap, servicer.GetUniqueID())
		}
	}

	if len(servicersMap) == 0 {
		delete(impl.servicers, servicerID)
	}
}

func (impl *servicerMDImpl) send4AllServicers(actID, bizID string, do func(defs.Servicer) error) {
	for servicerID, ss := range impl.servicers {
		for uniqueID, servicer := range ss {
			if actID != "" && len(servicer.GetActIDs()) > 0 && !slices.Contains(servicer.GetActIDs(), actID) {
				continue
			}

			if bizID != "" && len(servicer.GetBizIDs()) > 0 && !slices.Contains(servicer.GetBizIDs(), bizID) {
				continue
			}

			if err := do(servicer); err != nil {
				servicer.Remove("SendFailed")

				delete(ss, uniqueID)
			}
		}

		if len(ss) == 0 {
			delete(impl.servicers, servicerID)
		}
	}
}

func (impl *servicerMDImpl) send4AllOneServicer(servicerID uint64, do func(defs.Servicer) error) {
	servicers, ok := impl.servicers[servicerID]
	if !ok {
		return
	}

	for uniqueID, servicer := range servicers {
		if err := do(servicer); err != nil {
			servicer.Remove("SendFailed")

			delete(servicers, uniqueID)
		}
	}

	if len(servicers) == 0 {
		delete(impl.servicers, servicerID)
	}
}
