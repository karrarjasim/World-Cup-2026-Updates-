package store

import "time"

type User struct {
	ChatID        int64     `json:"chat_id"`
	Name          string    `json:"name"`
	Favorites     []string  `json:"favorites"`
	NotifyResults bool      `json:"notify_results"`
	NotifyDigest  bool      `json:"notify_digest"`
	NotifyNews    bool      `json:"notify_news"`
	SubscribedAt  time.Time `json:"subscribed_at"`
}

type Fixture struct {
	ID        int       `json:"id"`
	Kickoff   time.Time `json:"kickoff"`
	HomeID    int       `json:"home_id"`
	Home      string    `json:"home"`
	AwayID    int       `json:"away_id"`
	Away      string    `json:"away"`
	Round     string    `json:"round"`
	Status    string    `json:"status"`
	HomeGoals *int      `json:"home_goals"`
	AwayGoals *int      `json:"away_goals"`
}

func (f Fixture) Finished() bool {
	switch f.Status {
	case "FT", "AET", "PEN":
		return true
	}
	return false
}

func (f Fixture) Live() bool {
	switch f.Status {
	case "1H", "HT", "2H", "ET", "BT", "P", "SUSP", "INT", "LIVE":
		return true
	}
	return false
}

type GroupStanding struct {
	Group string        `json:"group"`
	Rows  []StandingRow `json:"rows"`
}

type StandingRow struct {
	Rank      int    `json:"rank"`
	TeamID    int    `json:"team_id"`
	Team      string `json:"team"`
	Played    int    `json:"played"`
	Win       int    `json:"win"`
	Draw      int    `json:"draw"`
	Lose      int    `json:"lose"`
	GoalsDiff int    `json:"goals_diff"`
	Points    int    `json:"points"`
}
