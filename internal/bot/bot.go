package bot

import (
	"context"
	"log"
	"strings"
	"time"

	"mundialbot/internal/assistant"
	"mundialbot/internal/config"
	"mundialbot/internal/football"
	"mundialbot/internal/format"
	"mundialbot/internal/store"
	"mundialbot/internal/telegram"
)

type Bot struct {
	cfg *config.Config
	tg  *telegram.Client
	st  *store.Store
	as  *assistant.Assistant
}

func New(cfg *config.Config, tg *telegram.Client, st *store.Store, as *assistant.Assistant) *Bot {
	return &Bot{cfg: cfg, tg: tg, st: st, as: as}
}

func (b *Bot) Run(ctx context.Context) {
	offset := 0
	for {
		if ctx.Err() != nil {
			return
		}
		updates, err := b.tg.GetUpdates(ctx, offset, 50)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("telegram getUpdates failed: %v", err)
			time.Sleep(3 * time.Second)
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message == nil || u.Message.Text == "" {
				continue
			}
			b.handle(ctx, u)
		}
	}
}

func (b *Bot) reply(ctx context.Context, chatID int64, html string) {
	if _, err := b.tg.Send(ctx, chatID, html); err != nil {
		log.Printf("send to %d failed: %v", chatID, err)
	}
}

func (b *Bot) handle(ctx context.Context, u telegram.Update) {
	chatID := u.Message.Chat.ID
	text := strings.TrimSpace(u.Message.Text)
	name := u.Message.From.FirstName

	if strings.HasPrefix(text, "/") {
		b.handleCommand(ctx, chatID, name, text)
		return
	}
	b.reply(ctx, chatID, b.as.Answer(ctx, text))
}

func (b *Bot) handleCommand(ctx context.Context, chatID int64, name, text string) {
	fields := strings.Fields(text)
	cmd := strings.ToLower(strings.TrimPrefix(fields[0], "/"))
	if i := strings.Index(cmd, "@"); i >= 0 {
		cmd = cmd[:i]
	}
	arg := strings.TrimSpace(strings.TrimPrefix(text, fields[0]))

	switch cmd {
	case "start":
		isNew := b.st.EnsureUser(chatID, name)
		if isNew {
			b.reply(ctx, chatID, welcome(name))
		} else {
			b.reply(ctx, chatID, "أنت مشترك بالفعل! ✅ ستصلك نتائج وتحليلات كأس العالم 2026 تلقائياً.\nاكتب /help لعرض الأوامر.")
		}
	case "help", "مساعدة":
		b.reply(ctx, chatID, helpText())
	case "today", "اليوم":
		b.reply(ctx, chatID, b.as.Answer(ctx, "مباريات اليوم"))
	case "standings", "ترتيب":
		b.reply(ctx, chatID, b.as.Answer(ctx, "ترتيب المجموعات"))
	case "next":
		if arg == "" {
			b.reply(ctx, chatID, "اكتب اسم المنتخب بعد الأمر، مثال: <code>/next البرازيل</code>")
			return
		}
		b.reply(ctx, chatID, b.as.Answer(ctx, "متى تلعب "+arg+" القادمة؟"))
	case "teams", "فرقي":
		if arg == "" {
			u, _ := b.st.GetUser(chatID)
			if len(u.Favorites) == 0 {
				b.reply(ctx, chatID, "لم تحدد فرقاً مفضلة بعد.\nمثال: <code>/teams البرازيل، الأرجنتين</code>")
			} else {
				b.reply(ctx, chatID, "فرقك المفضلة: <b>"+telegram.EscapeHTML(strings.Join(u.Favorites, "، "))+"</b>")
			}
			return
		}
		favs := splitTeams(arg)
		b.st.SetFavorites(chatID, favs)
		b.reply(ctx, chatID, "تم تحديث فرقك المفضلة ✅: <b>"+telegram.EscapeHTML(strings.Join(favs, "، "))+"</b>\nستصلك تذكيرات قبل مبارياتها.")
	case "news_off":
		b.st.SetNotify(chatID, "news", false)
		b.reply(ctx, chatID, "تم إيقاف إشعارات الأخبار 🔕")
	case "news_on":
		b.st.SetNotify(chatID, "news", true)
		b.reply(ctx, chatID, "تم تفعيل إشعارات الأخبار 🔔")
	case "stop", "الغاء":
		b.st.RemoveUser(chatID)
		b.reply(ctx, chatID, "تم إلغاء اشتراكك. يمكنك العودة في أي وقت بالضغط على /start. 👋")
	default:
		b.reply(ctx, chatID, "أمر غير معروف. اكتب /help لعرض الأوامر المتاحة.")
	}
}

func splitTeams(s string) []string {
	repl := strings.NewReplacer("،", ",", "؛", ",", "\n", ",")
	parts := strings.Split(repl.Replace(s), ",")
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func welcome(name string) string {
	greet := "أهلاً"
	if name != "" {
		greet += " " + telegram.EscapeHTML(name)
	}
	return greet + "! 🎉\n\n" +
		"أهلاً بك في <b>مساعد المونديال 2026</b> ⚽\n" +
		"كل التحديثات التلقائية تُنشر في قناتنا:\n" +
		"🏁 نتائج المباريات فور انتهائها مع تحليل ذكي\n" +
		"📅 ملخص يومي بمباريات اليوم\n" +
		"📰 أهم وأعجل الأخبار الرياضية\n\n" +
		"وهنا يمكنك سؤالي مباشرةً في أي وقت، مثل:\n" +
		"«منو متصدر المجموعة A؟» أو <code>/today</code> لمباريات اليوم.\n\n" +
		"اكتب /help لعرض كل الأوامر."
}

func helpText() string {
	return "<b>أوامر مساعد المونديال 2026</b> ⚽\n\n" +
		"/today — مباريات اليوم\n" +
		"/standings — ترتيب المجموعات\n" +
		"/next [منتخب] — موعد المباراة القادمة لمنتخب\n\n" +
		"📢 النتائج والأخبار والملخص اليومي تُنشر تلقائياً في القناة.\n" +
		"كما يمكنك سؤالي مباشرة بالعربية عن أي شيء يخص البطولة! 🧠"
}

func IsFavorite(f store.Fixture, favs []string) bool {
	for _, fav := range favs {
		fav = strings.ToLower(strings.TrimSpace(fav))
		if fav == "" {
			continue
		}
		for _, name := range []string{f.Home, f.Away} {
			if strings.Contains(strings.ToLower(name), fav) ||
				strings.Contains(strings.ToLower(football.ArName(name)), fav) {
				return true
			}
		}
	}
	return false
}

func PreMatchHTML(f store.Fixture, loc *time.Location) string {
	t := f.Kickoff.In(loc).Format("15:04")
	return "⏰ <b>تذكير: مباراة منتخبك بعد قليل!</b>\n" +
		format.FixtureLineHTML(f, loc) + "\n📍 الانطلاق الساعة " + t + " بتوقيت بغداد"
}
