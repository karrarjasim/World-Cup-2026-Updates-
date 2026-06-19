package config

import (
	"bufio"
	"errors"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	TelegramToken string

	ChannelID string

	FootballKey     string
	FootballBaseURL string

	LLMKey     string
	LLMModel   string
	LLMBaseURL string

	LeagueID int
	Season   int

	DataDir  string
	Location *time.Location

	AdminChatID int64

	DigestHour int

	LivePollSeconds int
}

func Load() (*Config, error) {
	loadDotEnv(".env")

	c := &Config{
		TelegramToken:   os.Getenv("TELEGRAM_TOKEN"),
		ChannelID:       strings.TrimSpace(os.Getenv("CHANNEL_ID")),
		FootballKey:     os.Getenv("FOOTBALL_API_KEY"),
		FootballBaseURL: envOr("FOOTBALL_BASE_URL", "https://site.api.espn.com"),
		LLMKey:          envOr("LLM_API_KEY", os.Getenv("GEMINI_API_KEY")),
		LLMModel:        envOr("LLM_MODEL", envOr("GEMINI_MODEL", "llama-3.3-70b-versatile")),
		LLMBaseURL:      envOr("LLM_BASE_URL", "https://api.groq.com/openai/v1"),
		DataDir:         envOr("DATA_DIR", "./data"),
		LeagueID:        envIntOr("LEAGUE_ID", 0),
		Season:          envIntOr("SEASON", 2026),
		DigestHour:      envIntOr("DIGEST_HOUR", 9),
		LivePollSeconds: envIntOr("LIVE_POLL_SECONDS", 240),
	}

	tz := envOr("TZ", "Asia/Baghdad")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	c.Location = loc

	if v := os.Getenv("ADMIN_CHAT_ID"); v != "" {
		c.AdminChatID, _ = strconv.ParseInt(v, 10, 64)
	}

	if c.TelegramToken == "" {
		return nil, errors.New("TELEGRAM_TOKEN is required")
	}
	if c.LLMKey == "" {
		log.Println("warning: no LLM_API_KEY set — AI analysis and chat answers will be disabled (scores, standings, reminders still work)")
	}
	if c.ChannelID == "" {
		log.Println("warning: no CHANNEL_ID set — automatic updates have nowhere to post")
	}
	return c, nil
}

func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.TrimSpace(line[:eq])
		val := strings.TrimSpace(line[eq+1:])
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
