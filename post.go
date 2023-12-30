package main

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostLink string

var (
	re                           = regexp.MustCompile(`https://vk\.com/wall(-[0-9]+)_([0-9]+)`)
	ErrPostCanOnlyContainOneLink = errors.New("a post can only contain one link")
)

func NewPostLink(link string) (PostLink, error) {
	ids := re.FindAllStringSubmatch(link, -1)
	if l := len(ids); l == 0 {
		return "", nil
	} else if l > 1 {
		return "", ErrPostCanOnlyContainOneLink
	}

	if _, err := strconv.Atoi(ids[0][1]); err != nil {
		return "", fmt.Errorf("failed to convert owner id %s: %w", link, err)
	}

	if _, err := strconv.Atoi(ids[0][2]); err != nil {
		return "", fmt.Errorf("failed to convert post id %s: %w", link, err)
	}

	return PostLink(strings.Join(ids[0][1:], "_")), nil
}

func (p PostLink) IDs() (string, string) {
	ids := strings.Split(string(p), "_")
	return ids[0], ids[1]
}

func (p PostLink) Link() string {
	return fmt.Sprintf("https://vk.com/wall%s", p)
}

type PostRepository struct {
	conn *pgxpool.Pool
}

func NewPostRepository(conn *pgxpool.Pool) *PostRepository {
	return &PostRepository{
		conn: conn,
	}
}

func (r *PostRepository) Create(ctx context.Context, p PostLink, peerID int) error {
	sql, args, err := squirrel.
		Insert("post").
		Columns("link", "peer_id").
		Values(p, peerID).
		Suffix("on conflict (link) do update set created_at = excluded.created_at").
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return fmt.Errorf("failed to build query: %w", err)
	}

	if _, err := r.conn.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}

func (r *PostRepository) GetLast(ctx context.Context, peerID int) ([]PostLink, error) {
	sql, args, err := squirrel.
		Select("link").
		From("post").
		Where("created_at > current_timestamp - '7 days'::interval").
		Where("peer_id = ?", peerID).
		OrderBy("created_at desc").
		PlaceholderFormat(squirrel.Dollar).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("failed to build query: %w", err)
	}

	rows, err := r.conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var posts []PostLink
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}

		posts = append(posts, PostLink(p))
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan posts: %w", err)
	}

	return posts, nil
}

func (r *PostRepository) Clear(ctx context.Context) error {
	const sql = `truncate table post cascade`

	if _, err := r.conn.Exec(ctx, sql); err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}

	return nil
}
