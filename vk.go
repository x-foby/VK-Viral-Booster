package main

import (
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/api/params"
	"go.uber.org/ratelimit"
)

type VKAdapter struct {
	vk *api.VK
	rl ratelimit.Limiter
}

func NewVKAdapter(vk *api.VK) *VKAdapter {
	return &VKAdapter{
		vk: vk,
		rl: ratelimit.New(20),
	}
}

func (a *VKAdapter) PostLiked(post PostLink, userID int) (bool, error) {
	const count = 100

	ownerID, postID := post.IDs()

	for i := 0; ; i++ {
		a.rl.Take()

		resp, err := a.vk.LikesGetList(api.Params{
			"type":     "post",
			"filter":   "likes",
			"owner_id": ownerID,
			"item_id":  postID,
			"count":    count,
			"offset":   i * count,
		})
		if err != nil {
			return false, fmt.Errorf("failed to fetch likes: %w", err)
		}

		if slices.Contains(resp.Items, userID) {
			return true, nil
		}

		if i*count+len(resp.Items) >= resp.Count {
			return false, nil
		}
	}
}

var (
	ErrUserNotFound = errors.New("user not found")
	ErrTooManyUsers = errors.New("too many users")
	ErrPostNotFound = errors.New("post not found")
	ErrTooManyPosts = errors.New("too many posts")
)

func (a *VKAdapter) PostExists(post PostLink) (bool, error) {
	a.rl.Take()

	resp, err := a.vk.WallGetByID(api.Params{
		"posts": post,
	})
	if err != nil {
		return false, err
	}

	if len(resp) == 0 {
		return false, ErrPostNotFound
	}

	if len(resp) > 1 {
		return false, ErrTooManyPosts
	}

	return true, nil
}

func (a *VKAdapter) UserNickname(id int) (string, error) {
	a.rl.Take()

	resp, err := a.vk.UsersGet(
		params.NewUsersGetBuilder().
			UserIDs([]string{strconv.Itoa(id)}).
			Fields([]string{"screen_name"}).
			Params)
	if err != nil {
		return "", err
	}

	if len(resp) == 0 {
		return "", ErrUserNotFound
	}

	if len(resp) > 1 {
		return "", ErrTooManyUsers
	}

	return resp[0].ScreenName, nil
}
