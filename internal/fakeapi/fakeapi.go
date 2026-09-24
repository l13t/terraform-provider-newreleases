// Package fakeapi is an in-memory fake of the newreleases.io API v1 used by the
// provider's acceptance tests (task testacc). It reproduces the behaviour the
// provider depends on, not the full service:
//
//   - responses are encoded with the client's own types, so optional fields
//     are omitted exactly like the real API (omitempty);
//   - project updates treat a missing or null field as "leave unchanged";
//   - email_notification "default" is resolved to a concrete schedule and
//     "none" is omitted from responses;
//   - unknown objects return 404, invalid input returns 400 with an errors
//     list, a wrong X-Key returns 401;
//   - the first write request is rejected once with 429 and Retry-After, so
//     every test run exercises the provider's retry transport;
//   - project listings are paginated one project per page.
//
// Anything the provider does not rely on (auth keys, release history paging,
// rate-limit headers) is not implemented.
package fakeapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"newreleases.io/newreleases"
)

// APIKey is the only key the fake accepts.
const APIKey = "fake-api-key"

// pageSize is deliberately tiny so pagination is exercised with few projects.
const pageSize = 1

var upstreamProviders = []string{"bitbucket", "cargo", "dockerhub", "gems", "github", "gitlab", "npm", "pypi"}

var emailNotifications = []newreleases.EmailNotification{
	newreleases.EmailNotificationNone,
	newreleases.EmailNotificationInstant,
	newreleases.EmailNotificationHourly,
	newreleases.EmailNotificationDaily,
	newreleases.EmailNotificationWeekly,
	newreleases.EmailNotificationDefault,
}

// Notification targets connected to the fake account.
var (
	slackChannels   = []newreleases.SlackChannel{{ID: "slack1", Channel: "releases", TeamName: "acme"}}
	telegramChats   = []newreleases.TelegramChat{{ID: "tg1", Type: "private", Name: "ops"}}
	discordChannels = []newreleases.DiscordChannel{{ID: "discord1", Name: "releases"}}
	matrixRooms     = []newreleases.MatrixRoom{{ID: "matrix1", Name: "ops", HomeserverURL: "https://matrix.org", InternalRoomID: "!ops:matrix.org"}}
	webhooks        = map[string][]newreleases.Webhook{
		"webhooks":                 {{ID: "hook1", Name: "deploy-bot"}},
		"hangouts-chat-webhooks":   {{ID: "hangouts1", Name: "releases"}},
		"microsoft-teams-webhooks": {{ID: "teams1", Name: "releases"}},
		"mattermost-webhooks":      {{ID: "mattermost1", Name: "releases"}},
		"rocketchat-webhooks":      {{ID: "rocketchat1", Name: "releases"}},
	}
)

// Server is a running fake API. Point the provider at URL with APIKey.
type Server struct {
	*httptest.Server

	mu            sync.Mutex
	seq           int
	projects      map[string]*newreleases.Project
	projectSeq    map[string]int
	tags          map[string]*newreleases.Tag
	rateLimitNext bool
}

// New starts a fake API server. Close it when done.
func New() *Server {
	s := &Server{
		projects:      map[string]*newreleases.Project{},
		projectSeq:    map[string]int{},
		tags:          map[string]*newreleases.Tag{},
		rateLimitNext: true,
	}
	s.Server = httptest.NewServer(http.HandlerFunc(s.serveHTTP))
	return s
}

func (s *Server) nextID(prefix string) string {
	s.seq++
	return fmt.Sprintf("%s%06d", prefix, s.seq)
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if r.Header.Get("X-Key") != APIKey {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet && s.rateLimitNext {
		s.rateLimitNext = false
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/v1/"), "/")
	parts := strings.Split(path, "/")
	switch parts[0] {
	case "providers":
		s.providers(w, r)
	case "slack-channels":
		writeJSON(w, map[string]any{"channels": slackChannels})
	case "telegram-chats":
		writeJSON(w, map[string]any{"chats": telegramChats})
	case "discord-channels":
		writeJSON(w, map[string]any{"channels": discordChannels})
	case "matrix-rooms":
		writeJSON(w, map[string]any{"rooms": matrixRooms})
	case "webhooks", "hangouts-chat-webhooks", "microsoft-teams-webhooks", "mattermost-webhooks", "rocketchat-webhooks":
		writeJSON(w, map[string]any{"webhooks": webhooks[parts[0]]})
	case "tags":
		s.tagsRoute(w, r, parts[1:])
	case "projects":
		s.projectsRoute(w, r, parts[1:])
	default:
		notFound(w)
	}
}

func (s *Server) providers(w http.ResponseWriter, r *http.Request) {
	if _, added := r.URL.Query()["added"]; !added {
		writeJSON(w, map[string]any{"providers": upstreamProviders})
		return
	}
	used := []string{}
	for _, p := range s.projects {
		if !slices.Contains(used, p.Provider) {
			used = append(used, p.Provider)
		}
	}
	slices.Sort(used)
	writeJSON(w, map[string]any{"providers": used})
}

func (s *Server) tagsRoute(w http.ResponseWriter, r *http.Request, parts []string) {
	var body struct {
		Name string `json:"name"`
	}
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			tags := make([]newreleases.Tag, 0, len(s.tags))
			for _, t := range s.tags {
				tags = append(tags, *t)
			}
			slices.SortFunc(tags, func(a, b newreleases.Tag) int { return strings.Compare(a.Name, b.Name) })
			writeJSON(w, map[string]any{"tags": tags})
		case http.MethodPost:
			if !decode(w, r, &body) {
				return
			}
			if body.Name == "" {
				badRequest(w, "name is required")
				return
			}
			t := &newreleases.Tag{ID: s.nextID("tag"), Name: body.Name}
			s.tags[t.ID] = t
			writeJSON(w, t)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	}

	t, ok := s.tags[parts[0]]
	if !ok || len(parts) > 1 {
		notFound(w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, t)
	case http.MethodPost:
		if !decode(w, r, &body) {
			return
		}
		if body.Name == "" {
			badRequest(w, "name is required")
			return
		}
		t.Name = body.Name
		writeJSON(w, t)
	case http.MethodDelete:
		delete(s.tags, t.ID)
		for _, p := range s.projects {
			p.TagIDs = slices.DeleteFunc(p.TagIDs, func(id string) bool { return id == t.ID })
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) projectsRoute(w http.ResponseWriter, r *http.Request, parts []string) {
	switch {
	case len(parts) == 0:
		switch r.Method {
		case http.MethodGet:
			s.listProjects(w, r, "")
		case http.MethodPost:
			s.addProject(w, r)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
		return
	case parts[0] == "search":
		s.searchProjects(w, r)
		return
	case slices.Contains(upstreamProviders, parts[0]) && len(parts) == 1:
		s.listProjects(w, r, parts[0])
		return
	}

	p, rest := s.resolveProject(parts)
	if p == nil {
		notFound(w)
		return
	}
	if len(rest) > 0 {
		s.releasesRoute(w, p, rest)
		return
	}
	s.projectRoute(w, r, p)
}

// resolveProject finds the project referenced by "<provider>/<name...>" or by
// ID, and returns the remaining path (a release sub-resource, if any).
func (s *Server) resolveProject(parts []string) (*newreleases.Project, []string) {
	if !slices.Contains(upstreamProviders, parts[0]) {
		return s.projects[parts[0]], parts[1:]
	}
	i := slices.IndexFunc(parts, func(seg string) bool { return seg == "releases" || seg == "latest-release" })
	if i < 0 {
		i = len(parts)
	}
	return s.projectByName(parts[0], strings.Join(parts[1:i], "/")), parts[i:]
}

func (s *Server) projectRoute(w http.ResponseWriter, r *http.Request, p *newreleases.Project) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, p)
	case http.MethodPost:
		var o newreleases.ProjectOptions
		if !decode(w, r, &o) {
			return
		}
		if errs := s.validateOptions(o); len(errs) > 0 {
			badRequest(w, errs...)
			return
		}
		applyOptions(p, o)
		writeJSON(w, p)
	case http.MethodDelete:
		delete(s.projects, p.ID)
		delete(s.projectSeq, p.ID)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) projectByName(provider, name string) *newreleases.Project {
	for _, p := range s.projects {
		if p.Provider == provider && p.Name == name {
			return p
		}
	}
	return nil
}

func (s *Server) addProject(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Provider string `json:"provider"`
		Name     string `json:"name"`
		newreleases.ProjectOptions
	}
	if !decode(w, r, &body) {
		return
	}
	var errs []string
	if !slices.Contains(upstreamProviders, body.Provider) {
		errs = append(errs, fmt.Sprintf("unsupported provider %q", body.Provider))
	}
	if body.Name == "" {
		errs = append(errs, "name is required")
	}
	errs = append(errs, s.validateOptions(body.ProjectOptions)...)
	if len(errs) == 0 && s.projectByName(body.Provider, body.Name) != nil {
		errs = append(errs, "project already added")
	}
	if len(errs) > 0 {
		badRequest(w, errs...)
		return
	}

	p := &newreleases.Project{
		ID:       s.nextID("prj"),
		Provider: body.Provider,
		Name:     body.Name,
		URL:      fmt.Sprintf("https://%s.example/%s/releases", body.Provider, body.Name),
	}
	if body.Provider == "github" {
		p.URL = "https://github.com/" + body.Name + "/releases"
	}
	applyOptions(p, body.ProjectOptions)
	s.projects[p.ID] = p
	s.projectSeq[p.ID] = s.seq
	writeJSON(w, p)
}

func (s *Server) validateOptions(o newreleases.ProjectOptions) []string {
	var errs []string
	if o.EmailNotification != nil && !slices.Contains(emailNotifications, *o.EmailNotification) {
		errs = append(errs, fmt.Sprintf("invalid email notification %q", *o.EmailNotification))
	}
	known := func(field string, ids []string, valid func(string) bool) {
		for _, id := range ids {
			if !valid(id) {
				errs = append(errs, fmt.Sprintf("unknown %s id %q", field, id))
			}
		}
	}
	webhookValid := func(kind string) func(string) bool {
		return func(id string) bool {
			return slices.ContainsFunc(webhooks[kind], func(h newreleases.Webhook) bool { return h.ID == id })
		}
	}
	known("slack channel", o.SlackIDs, func(id string) bool {
		return slices.ContainsFunc(slackChannels, func(c newreleases.SlackChannel) bool { return c.ID == id })
	})
	known("telegram chat", o.TelegramChatIDs, func(id string) bool {
		return slices.ContainsFunc(telegramChats, func(c newreleases.TelegramChat) bool { return c.ID == id })
	})
	known("discord channel", o.DiscordIDs, func(id string) bool {
		return slices.ContainsFunc(discordChannels, func(c newreleases.DiscordChannel) bool { return c.ID == id })
	})
	known("matrix room", o.MatrixRoomIDs, func(id string) bool {
		return slices.ContainsFunc(matrixRooms, func(m newreleases.MatrixRoom) bool { return m.ID == id })
	})
	known("webhook", o.WebhookIDs, webhookValid("webhooks"))
	known("hangouts chat webhook", o.HangoutsChatWebhookIDs, webhookValid("hangouts-chat-webhooks"))
	known("microsoft teams webhook", o.MSTeamsWebhookIDs, webhookValid("microsoft-teams-webhooks"))
	known("mattermost webhook", o.MattermostWebhookIDs, webhookValid("mattermost-webhooks"))
	known("rocketchat webhook", o.RocketchatWebhookIDs, webhookValid("rocketchat-webhooks"))
	known("tag", o.TagIDs, func(id string) bool { _, ok := s.tags[id]; return ok })
	for _, e := range o.Exclusions {
		if e.Value == "" {
			errs = append(errs, "exclusion value is required")
		}
	}
	return errs
}

// applyOptions updates p with every non-nil option, as the real API does.
func applyOptions(p *newreleases.Project, o newreleases.ProjectOptions) {
	if o.EmailNotification != nil {
		switch e := *o.EmailNotification; e {
		case newreleases.EmailNotificationDefault:
			p.EmailNotification = newreleases.EmailNotificationWeekly
		case newreleases.EmailNotificationNone:
			p.EmailNotification = ""
		default:
			p.EmailNotification = e
		}
	}
	set := func(dst *[]string, src []string) {
		if src != nil {
			*dst = slices.Clone(src)
		}
	}
	set(&p.SlackIDs, o.SlackIDs)
	set(&p.TelegramChatIDs, o.TelegramChatIDs)
	set(&p.DiscordIDs, o.DiscordIDs)
	set(&p.HangoutsChatWebhookIDs, o.HangoutsChatWebhookIDs)
	set(&p.MSTeamsWebhookIDs, o.MSTeamsWebhookIDs)
	set(&p.MattermostWebhookIDs, o.MattermostWebhookIDs)
	set(&p.RocketchatWebhookIDs, o.RocketchatWebhookIDs)
	set(&p.MatrixRoomIDs, o.MatrixRoomIDs)
	set(&p.WebhookIDs, o.WebhookIDs)
	set(&p.TagIDs, o.TagIDs)
	if o.Exclusions != nil {
		p.Exclusions = slices.Clone(o.Exclusions)
	}
	if o.ExcludePrereleases != nil {
		p.ExcludePrereleases = *o.ExcludePrereleases
	}
	if o.ExcludeUpdated != nil {
		p.ExcludeUpdated = *o.ExcludeUpdated
	}
	if o.Note != nil {
		p.Note = *o.Note
	}
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request, provider string) {
	q := r.URL.Query()
	tag := q.Get("tag")
	var all []newreleases.Project
	for _, p := range s.projects {
		if (provider == "" || p.Provider == provider) && (tag == "" || slices.Contains(p.TagIDs, tag)) {
			all = append(all, *p)
		}
	}
	slices.SortFunc(all, func(a, b newreleases.Project) int {
		if q.Get("order") == "name" {
			return strings.Compare(a.Name, b.Name)
		}
		return s.projectSeq[b.ID] - s.projectSeq[a.ID] // newest first
	})
	if _, ok := q["reverse"]; ok {
		slices.Reverse(all)
	}

	page := 1
	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			badRequest(w, "invalid page")
			return
		}
		page = n
	}
	totalPages := max((len(all)+pageSize-1)/pageSize, 1)
	start := min((page-1)*pageSize, len(all))
	end := min(start+pageSize, len(all))
	writeJSON(w, map[string]any{"projects": all[start:end], "total_pages": totalPages})
}

func (s *Server) searchProjects(w http.ResponseWriter, r *http.Request) {
	q, provider := r.URL.Query().Get("q"), r.URL.Query().Get("provider")
	out := []newreleases.Project{}
	for _, p := range s.projects {
		if strings.Contains(p.Name, q) && (provider == "" || p.Provider == provider) {
			out = append(out, *p)
		}
	}
	slices.SortFunc(out, func(a, b newreleases.Project) int { return strings.Compare(a.Name, b.Name) })
	writeJSON(w, map[string]any{"projects": out})
}

// releaseCatalog returns the releases of a project, newest first.
func releaseCatalog(p *newreleases.Project) []newreleases.Release {
	if p.Provider == "github" && p.Name == "golang/go" {
		return []newreleases.Release{
			{Version: "go1.25.1", Date: time.Date(2025, 9, 3, 16, 0, 0, 0, time.UTC), HasNote: true},
			{Version: "go1.25rc1", Date: time.Date(2025, 6, 11, 16, 0, 0, 0, time.UTC), IsPrerelease: true},
			{Version: "go1.21.0", Date: time.Date(2023, 8, 8, 16, 0, 0, 0, time.UTC), HasNote: true, CVE: []string{"CVE-2023-29409"}},
		}
	}
	return []newreleases.Release{
		{Version: "v1.0.0", Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)},
	}
}

func (s *Server) releasesRoute(w http.ResponseWriter, p *newreleases.Project, rest []string) {
	releases := releaseCatalog(p)
	switch {
	case len(rest) == 1 && rest[0] == "latest-release":
		for _, rel := range releases {
			if !p.ExcludePrereleases || !rel.IsPrerelease {
				writeJSON(w, rel)
				return
			}
		}
		notFound(w)
	case rest[0] == "releases" && (len(rest) == 2 || len(rest) == 3 && rest[2] == "note"):
		i := slices.IndexFunc(releases, func(rel newreleases.Release) bool { return rel.Version == rest[1] })
		if i < 0 {
			notFound(w)
			return
		}
		rel := releases[i]
		if len(rest) == 2 {
			writeJSON(w, rel)
			return
		}
		if !rel.HasNote {
			notFound(w)
			return
		}
		writeJSON(w, newreleases.ReleaseNote{
			Title:   rel.Version,
			Message: "<p>Release notes for " + rel.Version + ".</p>",
			URL:     p.URL,
		})
	default:
		notFound(w)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		badRequest(w, "invalid json: "+err.Error())
		return false
	}
	return true
}

func badRequest(w http.ResponseWriter, errs ...string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{"errors": errs})
}

func notFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]any{"message": "Not Found", "code": http.StatusNotFound})
}
