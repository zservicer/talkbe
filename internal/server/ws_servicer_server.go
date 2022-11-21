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
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func servicerWSReceive(conn *websocket.Conn, stream talkpb.ServiceTalkService_ServiceClient, logger l.Wrapper) {
	logger = logger.WithFields(l.StringField("func", "servicerWSReceiveRoutine"))

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

func servicerGRPCReceiveRoutine(stream talkpb.ServiceTalkService_ServiceClient, conn *websocket.Conn, logger l.Wrapper) {
	logger = logger.WithFields(l.StringField("func", "servicerGRPCReceiveRoutine"))

	logger.Debug("enter")
	defer logger.Debug("leave")
	defer conn.Close()

	for {
		resp, err := stream.Recv()
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("ReceiveFailed")

			if s, ok := status.FromError(err); ok {
				if s.Code() == codes.Unauthenticated {
					_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(int(s.Code()), s.Message()))
				}
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

func servicerWS(gRPCClient talkpb.ServiceTalkServiceClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
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

		stream, err := gRPCClient.Service(metadata.NewOutgoingContext(context.TODO(), md))
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("GRPCServiceFailed")

			return
		}

		go servicerGRPCReceiveRoutine(stream, c, logger)

		servicerWSReceive(c, stream, logger)
	}
}

type loginData struct {
	UserName string `json:"user_name"`
	Password string `json:"password"`
}

type loginDataResponse struct {
	Token    string `json:"token"`
	UserName string `json:"user_name"`
}

func loginHandler(gRPCClient talkpb.ServicerUserServicerClient, logger l.Wrapper) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		d, err := io.ReadAll(r.Body)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("ReadAllFailed")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		var loginD loginData

		err = json.Unmarshal(d, &loginD)
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("UnmarshalFailed")
			w.WriteHeader(http.StatusBadRequest)

			return
		}

		resp, err := gRPCClient.Login(r.Context(), &talkpb.LoginRequest{
			UserName: loginD.UserName,
			Password: loginD.Password,
		})
		if err != nil {
			logger.WithFields(l.ErrorField(err)).Error("LoginFailed")
			w.WriteHeader(http.StatusInternalServerError)

			return
		}

		if len(resp.ActIds) == 0 || len(resp.BizIds) == 0 {
			w.WriteHeader(http.StatusForbidden)

			return
		}

		ldResp := &loginDataResponse{
			Token:    resp.Token,
			UserName: resp.UserName,
		}

		d, _ = json.Marshal(ldResp)

		_, _ = w.Write(d)
	}
}
func SetupHTTPServicerServer(mux *http.ServeMux, cfg *config.WSConfig) {
	talkConn, err := clienttoolset.DialGRPC(cfg.ServicerGRPCClientConfig, []grpc.DialOption{
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(1024 * 1024 * 1024)),
	})
	if err != nil {
		cfg.Logger.Fatal(err)
	}

	// FIXME
	// defer talkConn.Close()

	gRPCTalkClient := talkpb.NewServiceTalkServiceClient(talkConn)

	userConn, err := clienttoolset.DialGRPC(cfg.ServicerUserGRPCClientConfig, nil)
	if err != nil {
		cfg.Logger.Fatal(err)
	}

	// FIXME
	// defer userConn.Close()

	gRPCUserClient := talkpb.NewServicerUserServicerClient(userConn)

	mux.HandleFunc("/login", loginHandler(gRPCUserClient, cfg.Logger))
	mux.HandleFunc("/ws/servicer", servicerWS(gRPCTalkClient, cfg.Logger))
}
