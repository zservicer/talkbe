package vo

import (
	"fmt"

	"github.com/sbasestarter/bizinters/talkinters"
	"github.com/zservicer/protorepo/gens/talkpb"
)

func TaskStatusMapPb2Db(status talkpb.TalkStatus) talkinters.TalkStatus {
	switch status {
	case talkpb.TalkStatus_TALK_STATUS_OPENED:
		return talkinters.TalkStatusOpened
	case talkpb.TalkStatus_TALK_STATUS_CLOSED:
		return talkinters.TalkStatusClosed
	default:
		return talkinters.TalkStatusNone
	}
}

func TaskStatusesMapPb2Db(statuses []talkpb.TalkStatus) []talkinters.TalkStatus {
	if statuses == nil {
		return nil
	}

	rStatues := make([]talkinters.TalkStatus, 0, len(statuses))

	for _, talkStatus := range statuses {
		rStatues = append(rStatues, TaskStatusMapPb2Db(talkStatus))
	}

	return rStatues
}

func TaskStatusMapDB2Pb(status talkinters.TalkStatus) talkpb.TalkStatus {
	switch status {
	case talkinters.TalkStatusOpened:
		return talkpb.TalkStatus_TALK_STATUS_OPENED
	case talkinters.TalkStatusClosed:
		return talkpb.TalkStatus_TALK_STATUS_CLOSED
	default:
		return talkpb.TalkStatus_TALK_STATUS_UNSPECIFIED
	}
}

func TalkInfoRDb2Pb(talkInfo *talkinters.TalkInfoR) *talkpb.TalkInfo {
	if talkInfo == nil {
		return nil
	}

	return &talkpb.TalkInfo{
		TalkId:       talkInfo.TalkID,
		Status:       TaskStatusMapDB2Pb(talkInfo.Status),
		Title:        talkInfo.Title,
		StartedAt:    uint64(talkInfo.StartAt),
		FinishedAt:   uint64(talkInfo.FinishedAt),
		CustomerName: talkInfo.CreatorUserName,
	}
}

func TalkInfoRsDB2Pb(talkInfos []*talkinters.TalkInfoR) []*talkpb.TalkInfo {
	if talkInfos == nil {
		return nil
	}

	rTalkInfos := make([]*talkpb.TalkInfo, 0, len(talkInfos))

	for _, info := range talkInfos {
		rTalkInfos = append(rTalkInfos, TalkInfoRDb2Pb(info))
	}

	return rTalkInfos
}

func TalkMessageWPb2Db(message *talkpb.TalkMessageW) *talkinters.TalkMessageW {
	if message == nil {
		return nil
	}

	dbMessage := &talkinters.TalkMessageW{}

	if message.GetText() != "" {
		dbMessage.Type = talkinters.TalkMessageTypeText
		dbMessage.Text = message.GetText()
	} else if len(message.GetImage()) > 0 {
		dbMessage.Type = talkinters.TalkMessageTypeImage
		dbMessage.Data = message.GetImage()
	} else {
		dbMessage.Type = talkinters.TalkMessageTypeUnknown
	}

	return dbMessage
}

func TalkMessageDB2Pb4Customer(message *talkinters.TalkMessageW) *talkpb.TalkMessage {
	pbMessage := talkMessageDB2Pb(message)
	if pbMessage != nil {
		if pbMessage.CustomerMessage {
			pbMessage.User = "您"
		} else {
			pbMessage.User = "客服"
		}
	}

	return pbMessage
}

func TalkMessageDB2Pb4Servicer(message *talkinters.TalkMessageW) *talkpb.TalkMessage {
	pbMessage := talkMessageDB2Pb(message)
	if pbMessage != nil {
		pbMessage.User = fmt.Sprintf("%s[%d]", pbMessage.User, message.SenderID)
	}

	return pbMessage
}

func talkMessageDB2Pb(message *talkinters.TalkMessageW) *talkpb.TalkMessage {
	if message == nil {
		return nil
	}

	pbMessage := &talkpb.TalkMessage{
		At:              uint64(message.At),
		CustomerMessage: message.CustomerMessage,
		User:            message.SenderUserName,
	}

	switch message.Type {
	case talkinters.TalkMessageTypeText:
		pbMessage.Message = &talkpb.TalkMessage_Text{
			Text: message.Text,
		}
	case talkinters.TalkMessageTypeImage:
		pbMessage.Message = &talkpb.TalkMessage_Image{
			Image: message.Data,
		}
	}

	return pbMessage
}

func TalkMessagesRDb2Pb(messages []*talkinters.TalkMessageR) []*talkpb.TalkMessage {
	if messages == nil {
		return nil
	}

	pbMessages := make([]*talkpb.TalkMessage, 0, len(messages))

	for _, message := range messages {
		pbMessages = append(pbMessages, TalkMessageDB2Pb4Servicer(&message.TalkMessageW))
	}

	return pbMessages
}
