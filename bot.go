package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/SevereCloud/vksdk/v2/events"
)

// var ErrPostCanOnlyContainOneLink = errors.New("a post can only contain one link")

type VK interface {
	PostExists(link PostLink) (bool, error)
	PostLiked(link PostLink, userID int) (bool, error)
	UserNickname(id int) (string, error)
}

type Chatter interface {
	Send(text string, peerID int) error
	ReplyTo(text string, messageID, peerID int) error
	Delete(messageID, peerID int) error
	IsAdmin(peerID, userID int) (bool, error)
}

type postRepo interface {
	Create(ctx context.Context, link PostLink, peerID int) error
	GetLast(ctx context.Context, peerID int) ([]PostLink, error)
}

type Bot struct {
	vk      VK
	chatter Chatter
	posts   postRepo
}

func NewBot(vk VK, chatter Chatter) *Bot {
	return &Bot{
		vk:      vk,
		chatter: chatter,
	}
}

func (b *Bot) WithPostRepo(r postRepo) *Bot {
	b.posts = r

	return b
}

const (
	commandStat = "stat"
)

func (b *Bot) MessageNew(ctx context.Context, e events.MessageNewObject) {
	var err error

	switch command(e.Message.Text) {
	case commandStat:
	}

	err = b.addPost(ctx, e)

	if err == nil {
		return
	}

	if err := b.chatter.ReplyTo(err.Error(), e.Message.ConversationMessageID, e.Message.PeerID); err != nil {
		log.Printf("failed to send error: %s\n", err)
	}
}

func (b *Bot) addPost(ctx context.Context, e events.MessageNewObject) error {
	link, err := NewPostLink(e.Message.Text)
	if err != nil {
		return err
	}
	if link == "" {
		return nil
	}

	if ok, err := b.vk.PostExists(link); !ok {
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to fetch post %s: %w", link, err)
	}

	allow, err := b.chatter.IsAdmin(e.Message.PeerID, e.Message.FromID)
	if err != nil {
		return fmt.Errorf("failed to get conversation info: %w", err)
	}

	if !allow {
		posts, err := b.posts.GetLast(ctx, e.Message.PeerID)
		if err != nil {
			return fmt.Errorf("failed to get last posts: %w", err)
		}

		allow, err = b.allowPost(ctx, e.Message.FromID, posts...)
		if err != nil {
			return fmt.Errorf("failed to get post statistics: %w", err)
		}
	}

	if !allow {
		if err := b.chatter.Delete(e.Message.ConversationMessageID, e.Message.PeerID); err != nil {
			return fmt.Errorf("failed to delete message: %w", err)
		}

		nickname, err := b.vk.UserNickname(e.Message.FromID)
		if err != nil {
			return fmt.Errorf("failed to get nickname for user %d: %w", e.Message.FromID, err)
		}

		if err := b.chatter.Send(fmt.Sprintf("Ссылка на пост удалена, так как @%s не лайкает чужие посты", nickname), e.Message.PeerID); err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		return nil
	}

	if err := b.posts.Create(ctx, link, e.Message.PeerID); err != nil {
		return fmt.Errorf("failed to create post %s: %w", link, err)
	}

	return nil
}

func (b *Bot) fromAdmin(ctx context.Context, userID int, posts ...PostLink) (bool, error) {
	return false, nil
}

func (b *Bot) allowPost(ctx context.Context, userID int, posts ...PostLink) (bool, error) {
	postsCount := float32(len(posts))

	if postsCount == 0 {
		return true, nil
	}

	threshold := postsCount * 0.8
	var count float32

	for _, p := range posts {
		liked, err := b.vk.PostLiked(p, userID)
		if err != nil {
			return false, err
		}
		if liked {
			count++
		}

		if count >= threshold {
			return true, nil
		}
	}

	return false, nil
}

func command(msg string) string {
	msg = strings.TrimSpace(msg)

	if len(msg) == 0 {
		return ""
	}

	if len(strings.Fields(msg)) != 1 {
		return ""
	}

	if msg[0] != '/' {
		return ""
	}

	return msg[1:]
}
