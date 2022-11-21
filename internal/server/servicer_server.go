package server

import (
	"fmt"
	"time"

	"github.com/godruoyi/go-snowflake"
	"github.com/sgostarter/i/l"
	"github.com/zservicer/protorepo/gens/talkpb"
	"github.com/zservicer/talkbe/internal/controller"
	"github.com/zservicer/talkbe/internal/defs"
	"github.com/zservicer/talkbe/internal/vo"
	"google.golang.org/grpc/codes"
)

func NewServicerServer(controller *controller.ServicerController, userTokenHelper defs.ServicerUserTokenHelper, model defs.ModelEx, logger l.Wrapper) talkpb.ServiceTalkServiceServer {
	if logger == nil {
		logger = l.NewNopLoggerWrapper()
	}

	if controller == nil || userTokenHelper == nil || model == nil {
		logger.Fatal("invalid input args")
	}

	return &servicerServerImpl{
		logger:          logger,
		controller:      controller,
		userTokenHelper: userTokenHelper,
		model:           model,
	}
}

type servicerServerImpl struct {
	talkpb.UnimplementedServiceTalkServiceServer

	logger          l.Wrapper
	userTokenHelper defs.ServicerUserTokenHelper
	model           defs.ModelEx

	controller *controller.ServicerController
}

func (impl *servicerServerImpl) Service(server talkpb.ServiceTalkService_ServiceServer) error {
	if server == nil {
		impl.logger.Error("noServerStream")

		return gRPCMessageError(codes.InvalidArgument, "noServerStream")
	}

	_, userID, userName, _, actIDs, bizIDs, err := impl.userTokenHelper.ExtractUserFromGRPCContext(server.Context(), false)
	if err != nil {
		return gRPCError(codes.Unauthenticated, err)
	}

	uniqueID := snowflake.ID()

	logger := impl.logger.WithFields(l.StringField(l.RoutineKey, "Service"),
		l.StringField("u", fmt.Sprintf("%d:%s", userID, userName)),
		l.UInt64Field("uniqueID", uniqueID))

	logger.Debug("enter")

	defer func() {
		logger.Debugf("leave")
	}()

	chSendMessage := make(chan *talkpb.ServiceResponse, 100)

	servicer := controller.NewServicer(userID, uniqueID, chSendMessage, actIDs, bizIDs)

	err = impl.controller.InstallServicer(servicer)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("InstallServicerFailed")

		return err
	}

	chTerminal := make(chan error, 2)

	go impl.serverReceiveRoutine(server, servicer, userID, userName, chTerminal, logger)

	loop := true

	for loop {
		select {
		case <-chTerminal:
			loop = false

			continue
		case message := <-chSendMessage:
			err = server.Send(message)

			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("SendMessageToStreamFailed")

				loop = false

				continue
			}

			if message.GetKickOut() != nil {
				logger.WithFields(l.ErrorField(err)).Error("KickOut")

				loop = false

				continue
			}
		}
	}

	err = impl.controller.UninstallServicer(servicer)
	if err != nil {
		logger.WithFields(l.ErrorField(err)).Error("UninstallServicerFailed")

		return err
	}

	return nil
}

//
//
//

func (impl *servicerServerImpl) serverReceiveRoutine(server talkpb.ServiceTalkService_ServiceServer,
	servicer defs.Servicer, userID uint64, userName string, chTerminal chan<- error, logger l.Wrapper) {
	var err error

	var request *talkpb.ServiceRequest

	defer func() {
		chTerminal <- err
	}()

	for {
		request, err = server.Recv()
		if err != nil {
			break
		}

		if request.GetAttachedTalks() != nil {
			err = impl.controller.ServicerQueryAttachedTalks(servicer)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("ServicerQueryAttachedTalksFailed")

				continue
			}
		} else if request.GetPendingTalks() != nil {
			err = impl.controller.ServicerQueryPendingTalks(servicer)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("ServicerQueryPendingTalks")

				continue
			}
		} else if reload := request.GetReload(); reload != nil {
			err = impl.controller.ServicerReloadTalk(servicer, reload.GetTalkId())
			if err != nil {
				logger.WithFields(l.ErrorField(err), l.StringField("talkID", reload.GetTalkId())).
					Error("ServicerReloadTalk")

				continue
			}
		} else if message := request.GetMessage(); message != nil {
			dbMessage := vo.TalkMessageWPb2Db(message.GetMessage())
			dbMessage.At = time.Now().Unix()
			dbMessage.CustomerMessage = false
			dbMessage.SenderID = userID
			dbMessage.SenderUserName = userName

			err = impl.model.AddTalkMessage(server.Context(), message.GetTalkId(), dbMessage)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("AddTalkMessageFailed")

				break
			}

			var seqID uint64
			if message.GetMessage() != nil {
				seqID = message.GetMessage().GetSeqId()
			}

			err = impl.controller.ServicerMessageIncoming(servicer, seqID, message.GetTalkId(), dbMessage)
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("CustomerMessageIncomingFailed")

				continue
			}
		} else if attach := request.GetAttach(); attach != nil {
			err = impl.controller.ServicerAttachTalk(servicer, attach.GetTalkId())

			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("AttachTalkFailed")

				continue
			}
		} else if detach := request.GetDetach(); detach != nil {
			err = impl.controller.ServicerDetachTalk(servicer, detach.GetTalkId())

			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("DetachTalkFailed")

				continue
			}
		} else {
			logger.Error("unknownMessage")
		}
	}
}
