package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/websocket"
	"github.com/sgostarter/i/l"
	"github.com/sgostarter/libservicetoolset/clienttoolset"
	"github.com/zservicer/protorepo/gens/talkpb"
	"github.com/zservicer/talkbe/config"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	httpTokenHeaderKey = "token"
)

func customerWSReceive(conn *websocket.Conn, stream talkpb.CustomerTalkService_TalkClient, logger l.Wrapper) {
	logger = logger.WithFields(l.StringField("func", "wsReceiveRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")

	for {
		messageType, message, err := conn.ReadMessage()
		if messageType == websocket.CloseMessage || err != nil {
			if err != nil {
				logger.WithFields(l.ErrorField(err)).Error("ReadMessageFailed")
			}

			break
		}

		var request talkpb.ServiceRequest

		err = proto.Unmarshal(message, &request)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UnmarshalFailed")

			break
		}

		err = stream.SendMsg(&request)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("SendGrpcMessageFailed")

			break
		}
	}
}

func customerGRPCReceiveRoutine(stream talkpb.CustomerTalkService_TalkClient, conn *websocket.Conn, logger l.Wrapper) {
	logger = logger.WithFields(l.StringField("func", "customerGRPCReceiveRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")
	defer conn.Close()

	for {
		resp, err := stream.Recv()
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("ReceiveFailed")

			if s, ok := status.FromError(err); ok {
				_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(int(s.Code()), s.Message()))
			}

			break
		}

		d, err := proto.Marshal(resp)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("MarshalFailed")

			break
		}

		err = conn.WriteMessage(websocket.BinaryMessage, d)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("WriteMessageFailed")

			break
		}
	}
}

func customerWS(gRPCClient talkpb.CustomerTalkServiceClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		Subprotocols: []string{"hey"},
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		logger = logger.WithFields(l.StringField(l.RoutineKey, "customerWS"))

		logger.Debug("enter")
		defer logger.Debug("leave")

		h := http.Header{}

		for _, sub := range websocket.Subprotocols(r) {
			if sub == "hey" {
				h.Set("Sec-Websocket-Protocol", "hey")

				break
			}
		}

		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UpgradeFailed")

			return
		}

		defer c.Close()

		_, msg, err := c.ReadMessage()
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("ReadTokenMessageFailed")

			return
		}

		kv := make(map[string]string)

		err = json.Unmarshal(msg, &kv)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UnmarshalTokenFailed")

			return
		}

		md := metadata.New(map[string]string{
			"token": kv["token"],
		})

		stream, err := gRPCClient.Talk(metadata.NewOutgoingContext(context.TODO(), md))
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("GRPCServiceFailed")

			return
		}

		go customerGRPCReceiveRoutine(stream, c, logger)

		customerWSReceive(c, stream, logger)
	}
}

func checkHandler(gRPCClient talkpb.CustomerUserServicerClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		md := metadata.New(map[string]string{
			"token": r.Header.Get(httpTokenHeaderKey),
		})

		resp, err := gRPCClient.CheckToken(metadata.NewOutgoingContext(context.TODO(), md), &talkpb.CheckTokenRequest{})
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("CheckTokenFailed")

			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		d, _ := proto.Marshal(resp)
		_, _ = w.Write(d)
	}
}

func createHandler(gRPCClient talkpb.CustomerUserServicerClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		d, err := io.ReadAll(r.Body)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("ReadAllFailed")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		var request talkpb.CreateTokenRequest

		err = proto.Unmarshal(d, &request)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UnmarshalFailed")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		resp, err := gRPCClient.CreateToken(context.TODO(), &request)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("CheckTokenFailed")

			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		d, _ = proto.Marshal(resp)
		_, _ = w.Write(d)
	}
}

func listTalkHandler(gRPCClient talkpb.CustomerTalkServiceClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		md := metadata.New(map[string]string{
			"token": r.Header.Get(httpTokenHeaderKey),
		})

		resp, err := gRPCClient.QueryTalks(metadata.NewOutgoingContext(context.TODO(), md), &talkpb.QueryTalksRequest{
			Statuses: []talkpb.TalkStatus{
				talkpb.TalkStatus_TALK_STATUS_OPENED,
			},
		})
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("CheckTokenFailed")

			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		d, _ := proto.Marshal(resp)
		_, _ = w.Write(d)
	}
}

func SetupHTTPCustomerServer(mux *http.ServeMux, cfg *config.WSConfig) {
	talkConn, err := clienttoolset.DialGRPC(cfg.CustomerGRPCClientConfig, nil)
	if err != nil {
		cfg.Logger.Fatal(err)
	}

	// FIXME
	// defer talkConn.Close()

	gRPCTalkClient := talkpb.NewCustomerTalkServiceClient(talkConn)

	//
	//
	//

	userConn, err := clienttoolset.DialGRPC(cfg.CustomerUserGRPCClientConfig, nil)
	if err != nil {
		cfg.Logger.Fatal(err)
	}

	// FIXME
	// defer userConn.Close()

	gRPCUserClient := talkpb.NewCustomerUserServicerClient(userConn)

	//
	//
	//

	mux.HandleFunc("/checkToken", checkHandler(gRPCUserClient, cfg.Logger))
	mux.HandleFunc("/createToken", createHandler(gRPCUserClient, cfg.Logger))
	mux.HandleFunc("/listTalk", listTalkHandler(gRPCTalkClient, cfg.Logger))
	mux.HandleFunc("/ws/customer", customerWS(gRPCTalkClient, cfg.Logger))
}
