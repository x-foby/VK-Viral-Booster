package main

import (
	"encoding/json"

	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/api/params"
)

type Chat struct {
	vk *api.VK
}

func NewChat(vk *api.VK) *Chat {
	return &Chat{
		vk: vk,
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
	_, err := c.vk.MessagesSend(params)
	return err
}

func (c *Chat) Delete(messageID, peerID int) error {
	_, err := c.vk.MessagesDelete(
		params.NewMessagesDeleteBuilder().
			ConversationMessageIDs([]int{messageID}).
			DeleteForAll(true).
			PeerID(peerID).
			Params,
	)
	return err
}

func (c *Chat) IsAdmin(peerID, userID int) (bool, error) {
	resp, err := c.vk.MessagesGetConversationMembers(
		params.
			NewMessagesGetConversationMembersBuilder().
			PeerID(peerID).
			Params,
	)

	for _, member := range resp.Items {
		if member.MemberID != userID {
			continue
		}

		return bool(member.IsAdmin), nil
	}

	return false, err
}
