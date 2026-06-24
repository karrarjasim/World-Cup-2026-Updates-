package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Embed the IANA timezone database in the binary so time.LoadLocation
	// (e.g. "Asia/Baghdad") always works, even on servers/containers that
	// ship without /usr/share/zoneinfo. Without this the bot silently falls
	// back to UTC and every announced kickoff is 3h early (wrong day for
	// late-night matches).
	_ "time/tzdata"

	"mundialbot/internal/assistant"
	"mundialbot/internal/bot"
	"mundialbot/internal/config"
	"mundialbot/internal/football"
	"mundialbot/internal/llm"
	"mundialbot/internal/scheduler"
	"mundialbot/internal/store"
	"mundialbot/internal/telegram"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	tg := telegram.New(cfg.TelegramToken)
	fb := football.New(cfg.FootballBaseURL, cfg.FootballKey, cfg.LeagueID, cfg.Season)
	ai := llm.New(cfg.LLMBaseURL, cfg.LLMKey, cfg.LLMModel)
	as := assistant.New(st, ai, cfg.Location)
	b := bot.New(cfg, tg, st, as)
	sch := scheduler.New(cfg, st, fb, ai, tg)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Printf("mundial bot starting (league=%d season=%d tz=%s)", cfg.LeagueID, cfg.Season, cfg.Location)

	go b.Run(ctx)
	go sch.Run(ctx)

	if cfg.AdminChatID != 0 {
		_, _ = tg.Send(ctx, cfg.AdminChatID, "✅ مساعد المونديال يعمل الآن.")
	}

	<-ctx.Done()
	log.Println("shutting down...")
	time.Sleep(500 * time.Millisecond)
}
