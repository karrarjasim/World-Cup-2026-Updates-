package football

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"mundialbot/internal/store"
)

type Client struct {
	base   string
	season int
	http   *http.Client
}

const leaguePath = "soccer/fifa.world"

func New(baseURL, _ string, _ int, season int) *Client {
	if baseURL == "" {
		baseURL = "https://site.api.espn.com"
	}
	return &Client{
		base:   strings.TrimRight(baseURL, "/"),
		season: season,
		http:   &http.Client{Timeout: 20 * time.Second},
	}
}

type scoreboardResp struct {
	Events []sbEvent `json:"events"`
}

type sbEvent struct {
	ID     string `json:"id"`
	Date   string `json:"date"`
	Status struct {
		Type struct {
			Name string `json:"name"`
		} `json:"type"`
	} `json:"status"`
	Competitions []struct {
		Notes []struct {
			Headline string `json:"headline"`
		} `json:"notes"`
		Competitors []struct {
			HomeAway string `json:"homeAway"`
			Score    string `json:"score"`
			Team     struct {
				ID          string `json:"id"`
				DisplayName string `json:"displayName"`
			} `json:"team"`
		} `json:"competitors"`
	} `json:"competitions"`
}

type standingsResp struct {
	Children []struct {
		Name      string `json:"name"`
		Standings struct {
			Entries []struct {
				Team struct {
					ID          string `json:"id"`
					DisplayName string `json:"displayName"`
				} `json:"team"`
				Stats []struct {
					Type  string  `json:"type"`
					Value float64 `json:"value"`
				} `json:"stats"`
			} `json:"entries"`
		} `json:"standings"`
	} `json:"children"`
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("espn api: status %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func parseDate(s string) time.Time {
	for _, layout := range []string{"2006-01-02T15:04Z07:00", time.RFC3339, "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func normStatus(name string) string {
	switch strings.ToUpper(name) {
	case "STATUS_SCHEDULED", "STATUS_PRE":
		return "NS"
	case "STATUS_FIRST_HALF", "STATUS_IN_PROGRESS":
		return "1H"
	case "STATUS_HALFTIME":
		return "HT"
	case "STATUS_SECOND_HALF":
		return "2H"
	case "STATUS_OVERTIME", "STATUS_END_OF_EXTRATIME", "STATUS_FIRST_EXTRA", "STATUS_SECOND_EXTRA":
		return "ET"
	case "STATUS_SHOOTOUT":
		return "PEN"
	case "STATUS_FULL_TIME", "STATUS_FINAL", "STATUS_FINAL_AET":
		return "FT"
	case "STATUS_POSTPONED":
		return "PST"
	case "STATUS_CANCELED", "STATUS_ABANDONED":
		return "CANC"
	}
	return name
}

func toFixture(e sbEvent) (store.Fixture, bool) {
	if len(e.Competitions) == 0 || len(e.Competitions[0].Competitors) < 2 {
		return store.Fixture{}, false
	}
	comp := e.Competitions[0]
	var home, away struct {
		id, score, name string
	}
	for _, c := range comp.Competitors {
		if c.HomeAway == "home" {
			home.id, home.score, home.name = c.Team.ID, c.Score, c.Team.DisplayName
		} else {
			away.id, away.score, away.name = c.Team.ID, c.Score, c.Team.DisplayName
		}
	}
	status := normStatus(e.Status.Type.Name)

	var hg, ag *int
	if status != "NS" && status != "PST" && status != "CANC" {
		if home.score != "" {
			n := atoi(home.score)
			hg = &n
		}
		if away.score != "" {
			n := atoi(away.score)
			ag = &n
		}
	}

	round := ""
	if len(comp.Notes) > 0 {
		round = comp.Notes[0].Headline
	}

	return store.Fixture{
		ID:        atoi(e.ID),
		Kickoff:   parseDate(e.Date),
		HomeID:    atoi(home.id),
		Home:      home.name,
		AwayID:    atoi(away.id),
		Away:      away.name,
		Round:     round,
		Status:    status,
		HomeGoals: hg,
		AwayGoals: ag,
	}, true
}

func (c *Client) scoreboard(ctx context.Context, from, to time.Time) ([]store.Fixture, error) {
	url := fmt.Sprintf("%s/apis/site/v2/sports/%s/scoreboard?dates=%s-%s&limit=500",
		c.base, leaguePath, from.Format("20060102"), to.Format("20060102"))
	var r scoreboardResp
	if err := c.getJSON(ctx, url, &r); err != nil {
		return nil, err
	}
	out := make([]store.Fixture, 0, len(r.Events))
	for _, e := range r.Events {
		if f, ok := toFixture(e); ok {
			out = append(out, f)
		}
	}
	return out, nil
}

func (c *Client) tournamentWindow() (time.Time, time.Time) {
	from := time.Date(c.season, time.June, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(c.season, time.July, 31, 0, 0, 0, 0, time.UTC)
	return from, to
}

func (c *Client) AllFixtures(ctx context.Context) ([]store.Fixture, error) {
	from, to := c.tournamentWindow()
	return c.scoreboard(ctx, from, to)
}

func (c *Client) FixturesByIDs(ctx context.Context, ids []int) ([]store.Fixture, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	want := make(map[int]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	all, err := c.AllFixtures(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]store.Fixture, 0, len(ids))
	for _, f := range all {
		if want[f.ID] {
			out = append(out, f)
		}
	}
	return out, nil
}

func stat(stats []struct {
	Type  string  `json:"type"`
	Value float64 `json:"value"`
}, key string) int {
	for _, s := range stats {
		if s.Type == key {
			return int(s.Value)
		}
	}
	return 0
}

func (c *Client) Standings(ctx context.Context) ([]store.GroupStanding, error) {
	url := fmt.Sprintf("%s/apis/v2/sports/%s/standings?season=%d", c.base, leaguePath, c.season)
	var r standingsResp
	if err := c.getJSON(ctx, url, &r); err != nil {
		return nil, err
	}
	out := make([]store.GroupStanding, 0, len(r.Children))
	for _, g := range r.Children {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			name = "المجموعة"
		}
		gs := store.GroupStanding{Group: name}
		for _, e := range g.Standings.Entries {
			gs.Rows = append(gs.Rows, store.StandingRow{
				Rank:      stat(e.Stats, "rank"),
				TeamID:    atoi(e.Team.ID),
				Team:      e.Team.DisplayName,
				Played:    stat(e.Stats, "gamesplayed"),
				Win:       stat(e.Stats, "wins"),
				Draw:      stat(e.Stats, "ties"),
				Lose:      stat(e.Stats, "losses"),
				GoalsDiff: stat(e.Stats, "pointdifferential"),
				Points:    stat(e.Stats, "points"),
			})
		}
		if len(gs.Rows) > 0 {
			out = append(out, gs)
		}
	}
	return out, nil
}
