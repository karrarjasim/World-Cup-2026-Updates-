package format

import (
	"fmt"
	"strings"
	"time"

	"mundialbot/internal/football"
	"mundialbot/internal/store"
	"mundialbot/internal/telegram"
)

func score(f store.Fixture) string {
	if f.HomeGoals == nil || f.AwayGoals == nil {
		return "-  —  -"
	}
	return fmt.Sprintf("%d  —  %d", *f.HomeGoals, *f.AwayGoals)
}

func ResultLineAR(f store.Fixture) string {
	return fmt.Sprintf("%s  %s  %s",
		football.ArName(f.Home), score(f), football.ArName(f.Away))
}

func ResultHTML(f store.Fixture) string {
	home := telegram.EscapeHTML(football.ArName(f.Home))
	away := telegram.EscapeHTML(football.ArName(f.Away))
	hg, ag := "-", "-"
	if f.HomeGoals != nil {
		hg = fmt.Sprintf("%d", *f.HomeGoals)
	}
	if f.AwayGoals != nil {
		ag = fmt.Sprintf("%d", *f.AwayGoals)
	}
	return fmt.Sprintf("🏁 <b>صافرة النهاية!</b>\n⚽ <b>%s [ %s ]  —  [ %s ] %s</b>",
		home, hg, ag, away)
}

func FixtureLineHTML(f store.Fixture, loc *time.Location) string {
	t := f.Kickoff.In(loc).Format("15:04")
	return fmt.Sprintf("• <b>%s</b>  vs  <b>%s</b> — الساعة %s",
		telegram.EscapeHTML(football.ArName(f.Home)),
		telegram.EscapeHTML(football.ArName(f.Away)), t)
}

func GroupTablePlain(g store.GroupStanding) string {
	var sb strings.Builder
	sb.WriteString(g.Group + "\n")
	for _, r := range g.Rows {
		sb.WriteString(fmt.Sprintf("%d) %s | لعب %d | نقاط %d | فارق %d\n",
			r.Rank, football.ArName(r.Team), r.Played, r.Points, r.GoalsDiff))
	}
	return sb.String()
}

func GroupTableHTML(g store.GroupStanding) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🏆 <b>%s</b>\n", telegram.EscapeHTML(g.Group)))
	for _, r := range g.Rows {
		sb.WriteString(fmt.Sprintf("%d. %s — <b>%d</b> نقطة (لعب %d، فارق %+d)\n",
			r.Rank, telegram.EscapeHTML(football.ArName(r.Team)), r.Points, r.Played, r.GoalsDiff))
	}
	return sb.String()
}
