package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/SevereCloud/vksdk/v2/events"
)

type VK interface {
	PostExists(link PostLink) (bool, error)
	UnlikedPosts(posts []PostLink, userID int) ([]PostLink, error)
	UserNickname(id int) (string, error)
}

type Chatter interface {
	Send(text string, peerID int) error
	ReplyTo(text string, messageID, peerID int) error
	Delete(messageID, peerID int) error
	MemberInfo(peerID, userID int) (chatMember, error)
}

type postRepo interface {
	Create(ctx context.Context, link PostLink, peerID int) error
	GetLast(ctx context.Context, peerID int, from time.Time) ([]PostLink, error)
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

type postLinks []PostLink

func (p postLinks) String() string {
	links := make([]string, len(p))

	for i := range p {
		links[i] = p[i].Link()
	}

	return strings.Join(links, ", ")
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

	member, err := b.chatter.MemberInfo(e.Message.PeerID, e.Message.FromID)
	if err != nil {
		return fmt.Errorf("failed to get conversation info: %w", err)
	}

	allow := member.IsAdmin

	var unliked []PostLink

	if !allow {
		posts, err := b.posts.GetLast(ctx, e.Message.PeerID, member.JoinedAt)
		if err != nil {
			return fmt.Errorf("failed to get last posts: %w", err)
		}

		unliked, err = b.unlikedPosts(ctx, e.Message.FromID, posts...)
		if err != nil {
			return fmt.Errorf("failed to get posts statistics: %w", err)
		}

		allow = len(unliked) == 0
	}

	if !allow {
		if err := b.chatter.Delete(e.Message.ConversationMessageID, e.Message.PeerID); err != nil {
			return fmt.Errorf("failed to delete message: %w", err)
		}

		nickname, err := b.vk.UserNickname(e.Message.FromID)
		if err != nil {
			return fmt.Errorf("failed to get nickname for user %d: %w", e.Message.FromID, err)
		}

		const message = `Ссылка на пост удалена, так как @%s не лайкает чужие посты.
Посты, которые необходимо лайкнуть: %s`

		if err := b.chatter.Send(fmt.Sprintf(message, nickname, postLinks(unliked)), e.Message.PeerID); err != nil {
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

func (b *Bot) unlikedPosts(ctx context.Context, userID int, posts ...PostLink) ([]PostLink, error) {
	postsCount := float32(len(posts))

	if postsCount == 0 {
		return nil, nil
	}

	unliked, err := b.vk.UnlikedPosts(posts, userID)
	if err != nil {
		return nil, err
	}

	if len(unliked) == 0 {
		return nil, nil
	}

	if postsCount-float32(len(unliked)) >= postsCount*0.8 {
		return nil, nil
	}

	return unliked, nil
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
