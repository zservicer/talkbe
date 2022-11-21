package impls

import (
	"context"
	"fmt"

	"github.com/sbasestarter/bizinters/talkinters"
	"github.com/sgostarter/i/l"
	"github.com/zservicer/protorepo/gens/talkpb"
	"github.com/zservicer/talkbe/internal/defs"
	"github.com/zservicer/talkbe/internal/vo"
)

func NewCustomerMD(mdi defs.CustomerMDI, logger l.Wrapper) defs.CustomerMD {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	impl := &customerMDImpl{
		mdi:       mdi,
		logger:    logger,
		customers: make(map[string]map[uint64]defs.Customer),
	}

	mdi.SetCustomerObserver(impl)

	return impl
}

type customerMDImpl struct {
	mrRunner defs.MainRoutineRunner
	mdi      defs.CustomerMDI
	logger   l.Wrapper

	customers map[string]map[uint64]defs.Customer // talkID - customerN - customer
}

//
// defs.Observer
//

func (impl *customerMDImpl) OnMessageIncoming(senderUniqueID uint64, talkID string, message *talkinters.TalkMessageW) {
	impl.mrRunner.Post(func() {
		impl.sendResponseToCustomers(0, talkID, &talkpb.TalkResponse{
			Talk: &talkpb.TalkResponse_Message{
				Message: vo.TalkMessageDB2Pb4Customer(message),
			},
		})
	})
}

func (impl *customerMDImpl) OnTalkClose(talkID string) {
	impl.mrRunner.Post(func() {
		impl.sendResponseToCustomers(0, talkID, &talkpb.TalkResponse{
			Talk: &talkpb.TalkResponse_Close{
				Close: &talkpb.TalkClose{},
			},
		})
	})
}

//
// defs.CustomerMD
//

func (impl *customerMDImpl) Setup(mr defs.MainRoutineRunner) {
	impl.mrRunner = mr
}

func (impl *customerMDImpl) InstallCustomer(ctx context.Context, customer defs.Customer) {
	if customer == nil {
		impl.logger.Error("noCustomer")

		return
	}

	ok, err := impl.mdi.GetM().TalkExists(ctx, []string{customer.GetActID()}, []string{customer.GetBizID()}, customer.GetTalkID())
	if err != nil {
		impl.logger.WithFields(l.StringField("talkID", customer.GetTalkID()), l.ErrorField(err)).
			Error("TalkNotExistsFailed")

		customer.Remove("TalkNotExists:" + err.Error())

		return
	}

	if !ok {
		impl.logger.WithFields(l.StringField("talkID", customer.GetTalkID())).Error("TalkNotExists")

		customer.Remove("TalkNotExists")

		return
	}

	err = impl.mdi.AddTrackTalk(ctx, customer.GetTalkID())
	if err != nil {
		impl.logger.WithFields(l.StringField("talkID", customer.GetTalkID()), l.ErrorField(err)).
			Error("AddTrackTalkFailed")

		customer.Remove("AddTrackTalkFailed:" + err.Error())

		return
	}

	if _, ok = impl.customers[customer.GetTalkID()]; !ok {
		impl.customers[customer.GetTalkID()] = make(map[uint64]defs.Customer)
	}

	impl.customers[customer.GetTalkID()][customer.GetUniqueID()] = customer

	if customer.CreateTalkFlag() {
		impl.mdi.SendTalkCreateMessage(customer.GetTalkID())
	}

	go func(logger l.Wrapper) {
		messages, errG := impl.mdi.GetM().GetTalkMessages(context.TODO(), customer.GetTalkID(), 0, 0)
		if errG != nil {
			logger.WithFields(l.ErrorField(errG)).Error("GetTalkMessageFailed")

			return
		}

		var pbMessages []*talkpb.TalkMessage

		for _, message := range messages {
			pbMessages = append(pbMessages, vo.TalkMessageDB2Pb4Customer(&message.TalkMessageW))
		}

		if errS := customer.SendMessage(&talkpb.TalkResponse{
			Talk: &talkpb.TalkResponse_Messages{
				Messages: &talkpb.TalkMessages{
					TalkId:   customer.GetTalkID(),
					Messages: pbMessages,
				},
			},
		}); errS != nil {
			logger.WithFields(l.ErrorField(errS)).Error("SendMessageFailed")
		}
	}(impl.logger.WithFields(l.StringField("customer", fmt.Sprintf("%s-%d", customer.GetTalkID(),
		customer.GetUniqueID()))))
}

func (impl *customerMDImpl) UninstallCustomer(ctx context.Context, customer defs.Customer) {
	if customer == nil {
		impl.logger.Error("noCustomer")

		return
	}

	impl.mdi.RemoveTrackTalk(ctx, customer.GetTalkID())

	if talkCustomers, ok := impl.customers[customer.GetTalkID()]; ok {
		delete(talkCustomers, customer.GetUniqueID())

		if len(talkCustomers) == 0 {
			delete(impl.customers, customer.GetTalkID())
		}
	}
}

func (impl *customerMDImpl) CustomerMessageIncoming(ctx context.Context,
	customer defs.Customer, seqID uint64, message *talkinters.TalkMessageW) {
	if customer == nil {
		impl.logger.Error("noCustomer")

		return
	}

	if message == nil {
		impl.logger.Error("noMessage")

		return
	}

	customersMap := impl.customers[customer.GetTalkID()]

	if err := customer.SendMessage(&talkpb.TalkResponse{
		Talk: &talkpb.TalkResponse_MessageConfirmed{
			MessageConfirmed: &talkpb.TalkMessageConfirmed{
				SeqId: seqID,
				At:    uint64(message.At),
			},
		},
	}); err != nil {
		impl.logger.WithFields(l.ErrorField(err), l.UInt64Field("id", customer.GetUniqueID())).
			Error("SendMessageFailed")

		customer.Remove("sendMessageFailed")

		delete(customersMap, customer.GetUniqueID())
	}

	if len(customersMap) == 0 {
		delete(impl.customers, customer.GetTalkID())
	}

	impl.mdi.SendMessage(customer.GetUniqueID(), customer.GetTalkID(), message)
}

func (impl *customerMDImpl) CustomerClose(ctx context.Context, customer defs.Customer) {
	if customer == nil {
		impl.logger.Error("noCustomer")

		return
	}

	if err := impl.mdi.GetM().CloseTalk(ctx, []string{customer.GetActID()}, []string{customer.GetBizID()},
		customer.GetTalkID()); err != nil {
		impl.logger.WithFields(l.ErrorField(err)).Error("CloseTalkFailed")

		return
	}

	impl.mdi.SendTalkCloseMessage(customer.GetTalkID())
}

//
//
//

func (impl *customerMDImpl) sendResponseToCustomers(excludedUniqueID uint64, talkID string, resp *talkpb.TalkResponse) {
	customersMap := impl.customers[talkID]

	for _, talkCustomer := range customersMap {
		if talkCustomer.GetUniqueID() == excludedUniqueID {
			continue
		}

		if err := talkCustomer.SendMessage(resp); err != nil {
			impl.logger.WithFields(l.ErrorField(err), l.UInt64Field("id", talkCustomer.GetUniqueID())).
				Error("SendMessageFailed")

			talkCustomer.Remove("sendMessageFailed")

			delete(customersMap, talkCustomer.GetUniqueID())
		}
	}

	if len(customersMap) == 0 {
		delete(impl.customers, talkID)
	}
}
