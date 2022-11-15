package server

import (
	"testing"

	"github.com/zservicer/protorepo/gens/talkpb"
	"google.golang.org/protobuf/encoding/protojson"
)

type utPrint struct {
	t  *testing.T
	id string
}

// nolint
func (print *utPrint) Print(msg string) {
	print.t.Logf("%s: %s", print.id, msg)
}

// nolint
func TestDestruct(t *testing.T) {
	print := &utPrint{
		t:  t,
		id: "1",
	}

	print.Print("enter")
	defer print.Print("leave")
	defer func() {
		print.Print("leave2")
	}()

	print = &utPrint{
		t:  t,
		id: "2",
	}

	print.Print("hoho")
}

func TestRequest(t *testing.T) {
	r := &talkpb.TalkRequest{
		Talk: &talkpb.TalkRequest_Create{
			Create: &talkpb.TalkCreateRequest{
				Title: "abcd",
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(r))

	r = &talkpb.TalkRequest{
		Talk: &talkpb.TalkRequest_Open{
			Open: &talkpb.TalkOpenRequest{
				TalkId: "635391d445a949658512eb4c",
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(r))

	r = &talkpb.TalkRequest{
		Talk: &talkpb.TalkRequest_Message{
			Message: &talkpb.TalkMessageW{
				SeqId: 10,
				Message: &talkpb.TalkMessageW_Text{
					Text: "1234",
				},
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(r))

	r = &talkpb.TalkRequest{
		Talk: &talkpb.TalkRequest_Close{
			Close: &talkpb.TalkClose{},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(r))

	sr := &talkpb.ServiceRequest{
		Request: &talkpb.ServiceRequest_Attach{
			Attach: &talkpb.ServiceAttachRequest{
				TalkId: "6358df94b02c89cb932c884f",
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(sr))

	sr = &talkpb.ServiceRequest{
		Request: &talkpb.ServiceRequest_Message{
			Message: &talkpb.ServicePostMessage{
				TalkId: "6358df94b02c89cb932c884f",
				Message: &talkpb.TalkMessageW{
					SeqId: 100,
					Message: &talkpb.TalkMessageW_Text{
						Text: "Who are you?",
					},
				},
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(sr))

	sr = &talkpb.ServiceRequest{
		Request: &talkpb.ServiceRequest_Detach{
			Detach: &talkpb.ServiceDetachRequest{
				TalkId: "6358df94b02c89cb932c884f",
			},
		},
	}

	t.Log(protojson.MarshalOptions{}.Format(sr))
}
