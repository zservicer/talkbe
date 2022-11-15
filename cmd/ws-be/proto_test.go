package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zservicer/protorepo/gens/talkpb"
	"google.golang.org/protobuf/proto"
)

func TestProto(t *testing.T) {
	r := &talkpb.TalkInfo{
		TalkId: "1",
		Title:  "title1",
	}

	d, err := proto.Marshal(r)
	assert.Nil(t, err)
	t.Log(d)
}
