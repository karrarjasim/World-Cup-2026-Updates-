package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	mu  sync.RWMutex
	dir string

	users     map[int64]*User
	fixtures  map[int]*Fixture
	sent      map[string]bool
	news      map[string]bool
	standings []GroupStanding
	meta      map[string]string
}

func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s := &Store{
		dir:      dir,
		users:    map[int64]*User{},
		fixtures: map[int]*Fixture{},
		sent:     map[string]bool{},
		news:     map[string]bool{},
		meta:     map[string]string{},
	}
	load(filepath.Join(dir, "users.json"), &s.users)
	load(filepath.Join(dir, "fixtures.json"), &s.fixtures)
	load(filepath.Join(dir, "sent.json"), &s.sent)
	load(filepath.Join(dir, "news.json"), &s.news)
	load(filepath.Join(dir, "standings.json"), &s.standings)
	load(filepath.Join(dir, "meta.json"), &s.meta)
	if s.users == nil {
		s.users = map[int64]*User{}
	}
	return s, nil
}

func load(path string, v any) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_ = json.Unmarshal(b, v)
}

func (s *Store) saveLocked(name string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return
	}
	final := filepath.Join(s.dir, name)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, final)
}

func (s *Store) EnsureUser(chatID int64, name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[chatID]; ok {
		return false
	}
	s.users[chatID] = &User{
		ChatID:        chatID,
		Name:          name,
		NotifyResults: true,
		NotifyDigest:  true,
		NotifyNews:    true,
		SubscribedAt:  time.Now().UTC(),
	}
	s.saveLocked("users.json", s.users)
	return true
}

func (s *Store) RemoveUser(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[chatID]; ok {
		delete(s.users, chatID)
		s.saveLocked("users.json", s.users)
	}
}

func (s *Store) GetUser(chatID int64) (User, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[chatID]
	if !ok {
		return User{}, false
	}
	return *u, true
}

func (s *Store) Users() []User {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, *u)
	}
	return out
}

func (s *Store) SetFavorites(chatID int64, favs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if u, ok := s.users[chatID]; ok {
		u.Favorites = favs
		s.saveLocked("users.json", s.users)
	}
}

func (s *Store) SetNotify(chatID int64, kind string, on bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[chatID]
	if !ok {
		return
	}
	switch kind {
	case "results":
		u.NotifyResults = on
	case "digest":
		u.NotifyDigest = on
	case "news":
		u.NotifyNews = on
	}
	s.saveLocked("users.json", s.users)
}

func (s *Store) UpsertFixtures(fs []Fixture) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range fs {
		f := fs[i]
		s.fixtures[f.ID] = &f
	}
	s.saveLocked("fixtures.json", s.fixtures)
}

func (s *Store) Fixtures() []Fixture {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Fixture, 0, len(s.fixtures))
	for _, f := range s.fixtures {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kickoff.Before(out[j].Kickoff) })
	return out
}

func (s *Store) Watchable(now time.Time) []Fixture {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Fixture
	for _, f := range s.fixtures {
		if f.Finished() {
			continue
		}
		start := f.Kickoff.Add(-2 * time.Minute)
		end := f.Kickoff.Add(160 * time.Minute)
		if (now.Equal(start) || now.After(start)) && now.Before(end) {
			out = append(out, *f)
		}
	}
	return out
}

func (s *Store) UpcomingForReminder(now time.Time, minLead, maxLead time.Duration) []Fixture {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Fixture
	lo, hi := now.Add(minLead), now.Add(maxLead)
	for _, f := range s.fixtures {
		if f.Status != "NS" && f.Status != "TBD" {
			continue
		}
		if f.Kickoff.After(lo) && f.Kickoff.Before(hi) {
			out = append(out, *f)
		}
	}
	return out
}

func (s *Store) FixturesOnLocalDay(day time.Time, loc *time.Location) []Fixture {
	y, m, d := day.In(loc).Date()
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Fixture
	for _, f := range s.fixtures {
		ky, km, kd := f.Kickoff.In(loc).Date()
		if ky == y && km == m && kd == d {
			out = append(out, *f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kickoff.Before(out[j].Kickoff) })
	return out
}

func (s *Store) NextMatchForTeam(query string, now time.Time) (Fixture, bool) {
	q := normalize(query)
	s.mu.RLock()
	defer s.mu.RUnlock()
	var best *Fixture
	for _, f := range s.fixtures {
		if f.Kickoff.Before(now) || f.Finished() {
			continue
		}
		if strings.Contains(normalize(f.Home), q) || strings.Contains(normalize(f.Away), q) {
			if best == nil || f.Kickoff.Before(best.Kickoff) {
				ff := *f
				best = &ff
			}
		}
	}
	if best == nil {
		return Fixture{}, false
	}
	return *best, true
}

func (s *Store) IsSent(fixtureID int, event string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sent[key(fixtureID, event)]
}

func (s *Store) MarkSent(fixtureID int, event string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent[key(fixtureID, event)] = true
	s.saveLocked("sent.json", s.sent)
}

func key(id int, event string) string { return fmt.Sprintf("%d:%s", id, event) }

func (s *Store) NewsSeen(id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.news[id]
}

func (s *Store) MarkNews(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.news[id] = true
	s.saveLocked("news.json", s.news)
}

func (s *Store) SetStandings(g []GroupStanding) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.standings = g
	s.saveLocked("standings.json", s.standings)
}

func (s *Store) Standings() []GroupStanding {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]GroupStanding, len(s.standings))
	copy(out, s.standings)
	return out
}

func (s *Store) GroupForTeam(teamID int, teamName string) (GroupStanding, bool) {
	q := normalize(teamName)
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, g := range s.standings {
		for _, r := range g.Rows {
			if (teamID != 0 && r.TeamID == teamID) || strings.Contains(normalize(r.Team), q) {
				return g, true
			}
		}
	}
	return GroupStanding{}, false
}

func (s *Store) MetaGet(k string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.meta[k]
}

func (s *Store) MetaSet(k, v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.meta[k] = v
	s.saveLocked("meta.json", s.meta)
}

func normalize(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
