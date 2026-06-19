package assistant

import (
	"context"
	"strings"
	"time"

	"mundialbot/internal/football"
	"mundialbot/internal/format"
	"mundialbot/internal/llm"
	"mundialbot/internal/store"
)

type Assistant struct {
	st  *store.Store
	ai  *llm.Client
	loc *time.Location
}

func New(st *store.Store, ai *llm.Client, loc *time.Location) *Assistant {
	return &Assistant{st: st, ai: ai, loc: loc}
}

func (a *Assistant) Answer(ctx context.Context, text string) string {
	q := strings.TrimSpace(text)
	low := strings.ToLower(q)
	now := time.Now().UTC()

	if containsAny(low, "ترتيب", "المجموعات", "المجموعه", "جدول", "standing") {
		groups := a.st.Standings()
		if len(groups) == 0 {
			return "ما زالت جداول الترتيب غير متوفرة بعد. سأوافيك بها فور انطلاق المباريات. ⚽"
		}
		for _, g := range groups {
			for _, r := range g.Rows {
				if strings.Contains(low, strings.ToLower(r.Team)) ||
					strings.Contains(q, football.ArName(r.Team)) {
					return format.GroupTableHTML(g)
				}
			}
		}
		var sb strings.Builder
		sb.WriteString("📊 <b>ترتيب المجموعات</b>\n\n")
		for _, g := range groups {
			sb.WriteString(format.GroupTableHTML(g))
			sb.WriteString("\n")
		}
		return strings.TrimSpace(sb.String())
	}

	if containsAny(low, "اليوم", "مباريات اليوم", "شنو يلعب", "ماكو مباريات") {
		todays := a.st.FixturesOnLocalDay(time.Now().In(a.loc), a.loc)
		if len(todays) == 0 {
			return "لا توجد مباريات مجدولة لليوم. 📅"
		}
		var sb strings.Builder
		sb.WriteString("📅 <b>مباريات اليوم</b>\n\n")
		for _, f := range todays {
			sb.WriteString(format.FixtureLineHTML(f, a.loc) + "\n")
		}
		return strings.TrimSpace(sb.String())
	}

	if containsAny(low, "متى", "شوكت", "القادمة", "الجاية", "القادمه", "next") {
		if team := guessTeam(q, a.st); team != "" {
			if f, ok := a.st.NextMatchForTeam(team, now); ok {
				t := f.Kickoff.In(a.loc).Format("Monday 2006-01-02 — 15:04")
				return "⏰ المباراة القادمة:\n<b>" + football.ArName(f.Home) + "</b> vs <b>" +
					football.ArName(f.Away) + "</b>\n🗓 " + t + " بتوقيت بغداد"
			}
			return "لم أجد مباراة قادمة لهذا المنتخب في الجدول الحالي. 🤔"
		}
	}

	answer, err := a.ai.Answer(ctx, q, a.contextSnapshot())
	if err != nil || answer == "" {
		return "تعذّر عليّ معالجة السؤال الآن. جرّب أوامر مثل: «ترتيب المجموعات»، «مباريات اليوم»، أو «متى تلعب البرازيل؟» ⚽"
	}
	return answer
}

func (a *Assistant) contextSnapshot() string {
	var sb strings.Builder
	todays := a.st.FixturesOnLocalDay(time.Now().In(a.loc), a.loc)
	if len(todays) > 0 {
		sb.WriteString("مباريات اليوم:\n")
		for _, f := range todays {
			sb.WriteString("- " + football.ArName(f.Home) + " ضد " + football.ArName(f.Away) +
				" (" + f.Kickoff.In(a.loc).Format("15:04") + ") الحالة: " + f.Status + "\n")
		}
		sb.WriteString("\n")
	}
	for _, g := range a.st.Standings() {
		sb.WriteString(format.GroupTablePlain(g))
		sb.WriteString("\n")
	}
	if sb.Len() == 0 {
		return "لا توجد بيانات محدّثة بعد."
	}
	return sb.String()
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

func guessTeam(text string, st *store.Store) string {
	low := strings.ToLower(text)
	for _, f := range st.Fixtures() {
		for _, name := range []string{f.Home, f.Away} {
			if strings.Contains(low, strings.ToLower(name)) {
				return name
			}
			if ar := football.ArName(name); ar != name && strings.Contains(text, ar) {
				return name
			}
		}
	}
	return ""
}
