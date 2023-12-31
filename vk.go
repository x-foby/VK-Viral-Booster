package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"text/template"

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
		rl: ratelimit.New(3),
	}
}

const vkScript = `var posts = [
    {{- range $post := .Posts }}
    {
        owner_id: {{ $post.OwnerID }},
        item_id: {{ $post.PostID }},
    },{{ end }}
];

var userId = {{ .UserID }};

var unliked = [];
var unchecked = [];
var totalRequestCount = 0;
var i = 0;

while (i < posts.length) {
    if (totalRequestCount < 25) {
        var j = 0;
        var inProgress = true;

        while (inProgress && totalRequestCount < 25) {
            var likes = API.likes.getList({
                type: "post",
                owner_id: posts[i].owner_id,
                item_id: posts[i].item_id,
                count: 1000,
                offset: j * 1000,
            });

            totalRequestCount = totalRequestCount + 1;

            if (likes.items.indexOf(userId) != -1) {
                inProgress = false;
            } else {
                if (j * 1000 + likes.items.length >= likes.count) {
                    inProgress = false;
                    unliked.push(posts[i]);
                } else {
                    j = j + 1;
                }
            }
        }
    } else {
        unchecked.push(posts[i]);
    }

    i = i + 1;
}

return {
    unliked: unliked,
    unchecked: unchecked,
};`

type vkPost struct {
	OwnerID string
	PostID  string
}

func (v *vkPost) UnmarshalJSON(data []byte) error {
	var raw struct {
		OwnerID int `json:"owner_id"`
		PostID  int `json:"item_id"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	v.OwnerID = strconv.Itoa(raw.OwnerID)
	v.PostID = strconv.Itoa(raw.PostID)

	return nil
}

type vkPosts []vkPost

func (p vkPosts) postLinks() []PostLink {
	l := make([]PostLink, len(p))

	for i := range p {
		l[i] = PostLink(fmt.Sprintf("%s_%s", p[i].OwnerID, p[i].PostID))
	}

	return l
}

func (p *vkPosts) append(posts ...vkPost) {
	idx := make(map[string]struct{})
	for _, post := range *p {
		idx[post.OwnerID+post.PostID] = struct{}{}
	}

	for _, post := range posts {
		if _, ok := idx[post.OwnerID+post.PostID]; ok {
			continue
		}

		*p = append(*p, post)
		idx[post.OwnerID+post.PostID] = struct{}{}
	}
}

func (a *VKAdapter) UnlikedPosts(posts []PostLink, userID int) ([]PostLink, error) {
	lenght := len(posts)

	if lenght == 0 {
		return nil, nil
	}

	tmplt, err := template.New("").Parse(vkScript)
	if err != nil {
		return nil, err
	}

	type response struct {
		Unliked   vkPosts `json:"unliked"`
		Unchecked vkPosts `json:"unchecked"`
	}

	var processed response
	for _, p := range posts {
		ownerID, postID := p.IDs()
		processed.Unchecked = append(processed.Unchecked, vkPost{
			OwnerID: ownerID,
			PostID:  postID,
		})
	}

	for {
		var buf bytes.Buffer

		if err := template.Must(tmplt.Clone()).Execute(&buf, map[string]interface{}{
			"Posts":  processed.Unchecked,
			"UserID": userID,
		}); err != nil {
			return nil, err
		}

		a.rl.Take()

		var resp response
		if err := executeWithRetries(a.vk, buf.String(), &resp); err != nil {
			return nil, fmt.Errorf("failed to fetch unliked posts: %w", err)
		}

		processed.Unliked.append(resp.Unliked...)
		processed.Unchecked = resp.Unchecked

		if len(processed.Unchecked) == 0 {
			break
		}
	}

	return processed.Unliked.postLinks(), nil
}

var (
	ErrUserNotFound = errors.New("user not found")
	ErrTooManyUsers = errors.New("too many users")
	ErrPostNotFound = errors.New("post not found")
	ErrTooManyPosts = errors.New("too many posts")
)

func (a *VKAdapter) PostExists(post PostLink) (bool, error) {
	a.rl.Take()

	resp, err := requestWithRetries(a.vk.WallGetByID, api.Params{
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

	resp, err := requestWithRetries(
		a.vk.UsersGet,
		params.NewUsersGetBuilder().
			UserIDs([]string{strconv.Itoa(id)}).
			Fields([]string{"screen_name"}).
			Params,
	)
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
