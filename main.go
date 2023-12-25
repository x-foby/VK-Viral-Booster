package main

import (
	"context"
	"log"
	"time"

	"github.com/SevereCloud/vksdk/v2/api"
	"github.com/SevereCloud/vksdk/v2/longpoll-bot"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kelseyhightower/envconfig"
)

func main() {
	var cfg config

	if err := envconfig.Process("", &cfg); err != nil {
		log.Fatal(err)
	}

	if err := cfg.validate(); err != nil {
		log.Fatalf("failed to validate config: %s\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgxpool.New(ctx, cfg.DataBase)
	if err != nil {
		log.Fatal(err)
	}

	vkbot := api.NewVK(cfg.APIKey)

	lp, err := longpoll.NewLongPoll(vkbot, cfg.GroupID)
	if err != nil {
		log.Fatal(err)
	}

	b := NewBot(NewVKAdapter(api.NewVK(cfg.Token)), NewChat(vkbot)).WithPostRepo(NewPostRepository(conn))

	lp.MessageNew(b.MessageNew)

	lp.Run()
}
