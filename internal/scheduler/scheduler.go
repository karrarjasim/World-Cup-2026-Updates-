package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"mundialbot/internal/bot"
	"mundialbot/internal/config"
	"mundialbot/internal/football"
	"mundialbot/internal/format"
	"mundialbot/internal/llm"
	"mundialbot/internal/store"
	"mundialbot/internal/telegram"
)

type Scheduler struct {
	cfg *config.Config
	st  *store.Store
	fb  *football.Client
	ai  *llm.Client
	tg  *telegram.Client

	lastLivePoll time.Time
}

func New(cfg *config.Config, st *store.Store, fb *football.Client, ai *llm.Client, tg *telegram.Client) *Scheduler {
	return &Scheduler{cfg: cfg, st: st, fb: fb, ai: ai, tg: tg}
}

func (s *Scheduler) Run(ctx context.Context) {
	s.dailySyncIfNeeded(ctx)

	tick := time.NewTicker(1 * time.Minute)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			s.dailySyncIfNeeded(ctx)
			s.maybePollLive(ctx)
			s.maybePreMatch(ctx)
			s.maybeDigest(ctx)
		}
	}
}

func (s *Scheduler) dailySyncIfNeeded(ctx context.Context) {
	today := time.Now().In(s.cfg.Location).Format("2006-01-02")
	if s.st.MetaGet("last_full_sync") == today {
		return
	}
	fixtures, err := s.fb.AllFixtures(ctx)
	if err != nil {
		log.Printf("daily fixture sync failed: %v", err)
		return
	}
	if len(fixtures) > 0 {
		s.st.UpsertFixtures(fixtures)
	}
	if standings, err := s.fb.Standings(ctx); err != nil {
		log.Printf("standings sync failed: %v", err)
	} else if len(standings) > 0 {
		s.st.SetStandings(standings)
	}
	s.st.MetaSet("last_full_sync", today)
	log.Printf("daily sync complete: %d fixtures", len(fixtures))
}

func (s *Scheduler) maybePollLive(ctx context.Context) {
	now := time.Now().UTC()
	watch := s.st.Watchable(now)
	if len(watch) == 0 {
		return
	}
	if time.Since(s.lastLivePoll) < time.Duration(s.cfg.LivePollSeconds)*time.Second {
		return
	}
	s.lastLivePoll = now

	ids := make([]int, 0, len(watch))
	for _, f := range watch {
		ids = append(ids, f.ID)
		if len(ids) == 20 {
			break
		}
	}
	updated, err := s.fb.FixturesByIDs(ctx, ids)
	if err != nil {
		log.Printf("live poll failed: %v", err)
		return
	}
	s.st.UpsertFixtures(updated)

	for _, f := range updated {
		if f.Finished() && !s.st.IsSent(f.ID, "ft") {
			s.broadcastResult(ctx, f)
			s.st.MarkSent(f.ID, "ft")
		}
	}
}

func (s *Scheduler) broadcastResult(ctx context.Context, f store.Fixture) {
	if standings, err := s.fb.Standings(ctx); err == nil && len(standings) > 0 {
		s.st.SetStandings(standings)
	}
	g, hasGroup := s.st.GroupForTeam(f.HomeID, f.Home)
	groupTable := "غير متوفر"
	if hasGroup {
		groupTable = format.GroupTablePlain(g)
	}

	analysis, err := s.ai.AnalyzeResult(ctx, format.ResultLineAR(f), groupTable)
	if err != nil {
		log.Printf("analysis failed for %d: %v", f.ID, err)
		analysis = ""
	}

	msg := format.ResultHTML(f)
	if r := strings.TrimSpace(f.Round); r != "" {
		msg += "\n🏆 " + telegram.EscapeHTML(translateRound(r))
	}
	if analysis != "" {
		msg += "\n\n🧠 <b>التحليل وحسابات التأهل:</b>\n" + telegram.EscapeHTML(analysis)
	}
	if hasGroup {
		msg += "\n\n📊 <b>ترتيب المجموعة بعد المباراة:</b>\n" + format.GroupTableHTML(g)
	}

	s.broadcastChannel(ctx, msg)
	log.Printf("broadcast result: %s %s", f.Home, f.Away)
}

func (s *Scheduler) maybePreMatch(ctx context.Context) {
	now := time.Now().UTC()
	upcoming := s.st.UpcomingForReminder(now, 55*time.Minute, 65*time.Minute)
	for _, f := range upcoming {
		if s.st.IsSent(f.ID, "pre") {
			continue
		}
		s.broadcastChannel(ctx, bot.PreMatchHTML(f, s.cfg.Location))
		s.st.MarkSent(f.ID, "pre")
	}
}

func (s *Scheduler) maybeDigest(ctx context.Context) {
	nowLocal := time.Now().In(s.cfg.Location)
	day := nowLocal.Format("2006-01-02")
	if nowLocal.Hour() != s.cfg.DigestHour {
		return
	}
	if s.st.MetaGet("last_digest") == day {
		return
	}
	s.st.MetaSet("last_digest", day)

	todays := s.st.FixturesOnLocalDay(nowLocal, s.cfg.Location)
	if len(todays) == 0 {
		return
	}
	var sb strings.Builder
	sb.WriteString("☀️ <b>صباح الخير! مباريات اليوم في المونديال</b>\n\n")
	for _, f := range todays {
		sb.WriteString(format.FixtureLineHTML(f, s.cfg.Location) + "\n")
	}
	sb.WriteString("\nبالتوفيق لمنتخباتكم! 🏆")
	s.broadcastChannel(ctx, strings.TrimSpace(sb.String()))
}

func (s *Scheduler) broadcastChannel(ctx context.Context, html string) {
	if s.cfg.ChannelID == "" {
		return
	}
	if err := s.tg.SendChannel(ctx, s.cfg.ChannelID, html); err != nil {
		log.Printf("channel post failed: %v", err)
	}
}

func translateRound(r string) string {
	low := strings.ToLower(r)
	switch {
	case strings.Contains(low, "group"):
		return "دور المجموعات — " + lastToken(r)
	case strings.Contains(low, "round of 16") || strings.Contains(low, "1/8"):
		return "دور الـ16"
	case strings.Contains(low, "quarter"):
		return "ربع النهائي"
	case strings.Contains(low, "semi"):
		return "نصف النهائي"
	case strings.Contains(low, "3rd place") || strings.Contains(low, "third place"):
		return "مباراة تحديد المركز الثالث"
	case strings.Contains(low, "final"):
		return "النهائي"
	}
	return r
}

func lastToken(s string) string {
	parts := strings.Fields(s)
	if len(parts) == 0 {
		return s
	}
	return fmt.Sprintf("الجولة %s", parts[len(parts)-1])
}
