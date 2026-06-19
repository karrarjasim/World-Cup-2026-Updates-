package news

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/xml"
	"io"
	"net/http"
	"strings"
	"time"
)

type Item struct {
	ID      string
	Title   string
	Link    string
	Snippet string
}

type Fetcher struct {
	feeds        []string
	worldCupOnly bool
	http         *http.Client
}

func New(feeds []string, worldCupOnly bool) *Fetcher {
	return &Fetcher{feeds: feeds, worldCupOnly: worldCupOnly, http: &http.Client{Timeout: 20 * time.Second}}
}

var keywords = []string{
	"كأس العالم", "كاس العالم", "المونديال", "مونديال", "world cup", "2026", "فيفا", "fifa",
}

func (f *Fetcher) Fetch(ctx context.Context) []Item {
	var out []Item
	seen := map[string]bool{}
	for _, url := range f.feeds {
		items, err := f.fetchOne(ctx, url)
		if err != nil {
			continue
		}
		for _, it := range items {
			if f.worldCupOnly && !relevant(it.Title+" "+it.Snippet) {
				continue
			}
			if seen[it.ID] {
				continue
			}
			seen[it.ID] = true
			out = append(out, it)
		}
	}
	return out
}

func relevant(text string) bool {
	low := strings.ToLower(text)
	for _, k := range keywords {
		if strings.Contains(low, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

type rss struct {
	Channel struct {
		Items []struct {
			Title       string `xml:"title"`
			Link        string `xml:"link"`
			GUID        string `xml:"guid"`
			Description string `xml:"description"`
		} `xml:"item"`
	} `xml:"channel"`
}

type atom struct {
	Entries []struct {
		Title string `xml:"title"`
		ID    string `xml:"id"`
		Links []struct {
			Href string `xml:"href,attr"`
			Rel  string `xml:"rel,attr"`
		} `xml:"link"`
		Summary string `xml:"summary"`
	} `xml:"entry"`
}

func (f *Fetcher) fetchOne(ctx context.Context, url string) ([]Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "MundialBot/1.0")
	resp, err := f.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	var r rss
	if err := xml.Unmarshal(body, &r); err == nil && len(r.Channel.Items) > 0 {
		var out []Item
		for _, it := range r.Channel.Items {
			id := it.GUID
			if id == "" {
				id = it.Link
			}
			out = append(out, Item{
				ID:      hashID(id),
				Title:   strings.TrimSpace(it.Title),
				Link:    strings.TrimSpace(it.Link),
				Snippet: stripTags(it.Description),
			})
		}
		return out, nil
	}

	var a atom
	if err := xml.Unmarshal(body, &a); err == nil && len(a.Entries) > 0 {
		var out []Item
		for _, e := range a.Entries {
			link := ""
			for _, l := range e.Links {
				if l.Rel == "" || l.Rel == "alternate" {
					link = l.Href
					break
				}
			}
			id := e.ID
			if id == "" {
				id = link
			}
			out = append(out, Item{
				ID:      hashID(id),
				Title:   strings.TrimSpace(e.Title),
				Link:    strings.TrimSpace(link),
				Snippet: stripTags(e.Summary),
			})
		}
		return out, nil
	}
	return nil, nil
}

func hashID(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func stripTags(s string) string {
	var sb strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				sb.WriteRune(r)
			}
		}
	}
	out := strings.TrimSpace(sb.String())
	if len(out) > 280 {
		out = out[:280]
	}
	return out
}
