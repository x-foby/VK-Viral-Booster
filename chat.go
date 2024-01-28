package main

import (
	"encoding/json"
	"time"

	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/api/params"
	"go.uber.org/ratelimit"
)

type Chat struct {
	vk *api.VK
	rl ratelimit.Limiter
}

func NewChat(vk *api.VK) *Chat {
	return &Chat{
		vk: vk,
		rl: ratelimit.New(10),
	}
}

func (c *Chat) Send(text string, peerID int) error {
	return c.send(
		params.
			NewMessagesSendBuilder().
			Message(text).
			RandomID(0).
			PeerID(peerID).
			Params,
	)
}

func (c *Chat) ReplyTo(text string, messageID, peerID int) error {
	buf, err := json.Marshal(map[string]interface{}{
		"peer_id":                  peerID,
		"conversation_message_ids": messageID,
		"is_reply":                 1,
	})
	if err != nil {
		return err
	}

	return c.send(
		params.
			NewMessagesSendBuilder().
			Message(text).
			RandomID(0).
			PeerID(peerID).
			// ReplyTo(messageID).
			Forward(string(buf)).
			Params,
	)
}

func (c *Chat) send(params api.Params) error {
	c.rl.Take()

	_, err := requestWithRetries(c.vk.MessagesSend, params)
	return err
}

func (c *Chat) Delete(messageID, peerID int) error {
	c.rl.Take()

	_, err := requestWithRetries(
		c.vk.MessagesDelete,
		params.NewMessagesDeleteBuilder().
			ConversationMessageIDs([]int{messageID}).
			DeleteForAll(true).
			PeerID(peerID).
			Params,
	)
	return err
}

type chatMember struct {
	JoinedAt time.Time
	IsAdmin  bool
}

func (c *Chat) MemberInfo(peerID, userID int) (chatMember, error) {
	c.rl.Take()

	resp, err := requestWithRetries(
		c.vk.MessagesGetConversationMembers,
		params.
			NewMessagesGetConversationMembersBuilder().
			PeerID(peerID).
			Params,
	)

	for _, member := range resp.Items {
		if member.MemberID != userID {
			continue
		}

		return chatMember{
			JoinedAt: time.Unix(int64(member.JoinDate), 0),
			IsAdmin:  bool(member.IsAdmin),
		}, nil
	}

	return chatMember{}, err
}
