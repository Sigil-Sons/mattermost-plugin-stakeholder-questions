package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// ServeHTTP routes /plugins/com.mysticspark.stakeholder-questions/api/v1/...
// Auth: Mattermost sets Mattermost-User-ID for session users and bot/personal access tokens.
func (p *Plugin) ServeHTTP(_ *plugin.Context, w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("Mattermost-User-ID")
	if userID == "" {
		writeErr(w, http.StatusUnauthorized, "not authenticated")
		return
	}
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch {
	case path == "/api/v1/packs" && r.Method == http.MethodPost:
		p.handleCreate(w, r, userID)
	case path == "/api/v1/me/pending" && r.Method == http.MethodGet:
		p.handlePending(w, r, userID)
	case strings.HasPrefix(path, "/api/v1/packs/") && r.Method == http.MethodGet:
		id := strings.TrimPrefix(path, "/api/v1/packs/")
		if strings.Contains(id, "/") {
			writeErr(w, http.StatusNotFound, "not found")
			return
		}
		p.handleGet(w, r, userID, id)
	default:
		writeErr(w, http.StatusNotFound, "not found")
	}
}

func (p *Plugin) resolveUser(ref string) (*model.User, error) {
	ref = strings.TrimPrefix(strings.TrimSpace(ref), "@")
	if ref == "" {
		return nil, fmt.Errorf("empty respondent")
	}
	if model.IsValidId(ref) {
		if u, appErr := p.API.GetUser(ref); appErr == nil {
			return u, nil
		}
	}
	u, appErr := p.API.GetUserByUsername(strings.ToLower(ref))
	if appErr != nil {
		return nil, fmt.Errorf("unknown user %q", ref)
	}
	return u, nil
}

func (p *Plugin) handleCreate(w http.ResponseWriter, r *http.Request, userID string) {
	var req CreateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json: "+err.Error())
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" || req.ChannelID == "" {
		writeErr(w, http.StatusBadRequest, "title and channel_id are required")
		return
	}
	cfg := p.config()
	ch, appErr := p.API.GetChannel(req.ChannelID)
	if appErr != nil {
		writeErr(w, http.StatusNotFound, "channel not found")
		return
	}
	if !cfg.teamAllowed(ch.TeamId) {
		writeErr(w, http.StatusForbidden, "packs are not enabled for this team")
		return
	}
	if !p.API.HasPermissionToChannel(userID, ch.Id, model.PermissionCreatePost) {
		writeErr(w, http.StatusForbidden, "no permission to post in this channel")
		return
	}
	if err := normalizeQuestions(req.Questions, cfg.maxQuestions()); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(req.Respondents) == 0 {
		writeErr(w, http.StatusBadRequest, "at least one respondent is required")
		return
	}
	var ids, names []string
	for _, ref := range req.Respondents {
		u, err := p.resolveUser(ref)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		if contains(ids, u.Id) {
			continue
		}
		if _, appErr := p.API.GetChannelMember(ch.Id, u.Id); appErr != nil {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("@%s is not a member of this channel", u.Username))
			return
		}
		ids = append(ids, u.Id)
		names = append(names, u.Username)
	}
	imp := strings.ToLower(strings.TrimSpace(req.Importance))
	if imp == "" {
		imp = "normal"
	}
	if imp != "low" && imp != "normal" && imp != "high" {
		writeErr(w, http.StatusBadRequest, "importance must be low, normal or high")
		return
	}
	author, appErr := p.API.GetUser(userID)
	if appErr != nil {
		writeErr(w, http.StatusInternalServerError, "author lookup failed")
		return
	}
	now := model.GetMillis()
	closes := req.ClosesAt
	if closes == nil && cfg.DefaultCloseDays > 0 {
		c := now + int64(cfg.DefaultCloseDays)*24*60*60*1000
		closes = &c
	}
	pk := &Pack{
		ID: "pack_" + model.NewId(), TeamID: ch.TeamId, ChannelID: ch.Id,
		Title: req.Title, Summary: strings.TrimSpace(req.Summary),
		AuthorUserID: userID, AuthorBot: author.IsBot,
		RespondentUserIDs: ids, MinRespondents: defaultMinRespondents(req.MinRespondents, len(ids), imp),
		Importance: imp, Visibility: VisibilityOthersOptIn,
		AllowSkip: req.AllowSkip, AllowReviseUntilLock: true,
		Status: StatusOpen, CreatedAt: now, ClosesAt: closes,
		Questions: req.Questions, Answers: map[string]map[string]Answer{}, CompletedUserIDs: []string{},
	}

	p.kvLock.Lock()
	defer p.kvLock.Unlock()
	if err := p.savePack(pk); err != nil {
		writeErr(w, http.StatusInternalServerError, "store failed")
		return
	}
	post := &model.Post{
		UserId: userID, ChannelId: ch.Id, Type: PostType,
		Message: fallbackMessage(pk, names),
	}
	post.SetProps(cardProps(pk, names, nil))
	created, appErr := p.API.CreatePost(post)
	if appErr != nil {
		_ = p.API.KVDelete(packKey(pk.ID))
		writeErr(w, http.StatusInternalServerError, "create post failed: "+appErr.Error())
		return
	}
	pk.PostID = created.Id
	if err := p.savePack(pk); err != nil {
		writeErr(w, http.StatusInternalServerError, "store failed")
		return
	}
	_ = p.indexAdd(channelIndexKey(ch.Id), pk.ID)
	for _, id := range ids {
		_ = p.indexAdd(userPendingKey(id), pk.ID)
	}
	writeJSON(w, http.StatusCreated, pk)
}

// fallbackMessage is what non-plugin clients (mobile without webapp, email, search) see; it @mentions respondents (G10).
func fallbackMessage(pk *Pack, names []string) string {
	at := make([]string, len(names))
	for i, n := range names {
		at[i] = "@" + n
	}
	return fmt.Sprintf("📋 **Stakeholder pack · %s** — %s, %d question(s) waiting on you.",
		pk.Title, strings.Join(at, " "), len(pk.Questions))
}

// cardProps carries everything the channel card needs to render without an extra fetch.
func cardProps(pk *Pack, names []string, completedNames []string) model.StringInterface {
	if completedNames == nil {
		completedNames = []string{}
	}
	return model.StringInterface{
		"ssq_pack_id":         pk.ID,
		"ssq_title":           pk.Title,
		"ssq_summary":         pk.Summary,
		"ssq_status":          pk.Status,
		"ssq_question_count":  len(pk.Questions),
		"ssq_min_respondents": pk.MinRespondents,
		"ssq_respondents":     names,
		"ssq_completed":       completedNames,
		"ssq_created_at":      pk.CreatedAt,
	}
}

func (p *Plugin) viewerFor(pk *Pack, userID string, r *http.Request) (viewer, bool) {
	v := viewer{
		UserID:       userID,
		IsAuthor:     pk.AuthorUserID == userID,
		IsRespondent: contains(pk.RespondentUserIDs, userID),
		ShowOthers:   r.URL.Query().Get("show_others") == "1",
	}
	v.IsAdmin = p.API.HasPermissionTo(userID, model.PermissionManageSystem) ||
		p.API.HasPermissionToTeam(userID, pk.TeamID, model.PermissionManageTeam)
	canSee := v.IsAuthor || v.IsAdmin || v.IsRespondent ||
		p.API.HasPermissionToChannel(userID, pk.ChannelID, model.PermissionReadChannel)
	return v, canSee
}

func (p *Plugin) handleGet(w http.ResponseWriter, r *http.Request, userID, id string) {
	pk, err := p.getPack(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load failed")
		return
	}
	if pk == nil {
		writeErr(w, http.StatusNotFound, "pack not found")
		return
	}
	v, ok := p.viewerFor(pk, userID, r)
	if !ok {
		writeErr(w, http.StatusNotFound, "pack not found")
		return
	}
	writeJSON(w, http.StatusOK, visibleCopy(pk, v, p.config().AllowObserverResultsOnComplete))
}

type pendingItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ChannelID string `json:"channel_id"`
	PostID    string `json:"post_id"`
	Questions int    `json:"questions"`
	Answered  int    `json:"answered"`
}

func (p *Plugin) handlePending(w http.ResponseWriter, _ *http.Request, userID string) {
	ids, err := p.getIndex(userPendingKey(userID))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load failed")
		return
	}
	out := []pendingItem{}
	for _, id := range ids {
		pk, err := p.getPack(id)
		if err != nil || pk == nil || pk.Status != StatusOpen || contains(pk.CompletedUserIDs, userID) {
			continue
		}
		out = append(out, pendingItem{ID: pk.ID, Title: pk.Title, ChannelID: pk.ChannelID, PostID: pk.PostID,
			Questions: len(pk.Questions), Answered: len(pk.Answers[userID])})
	}
	writeJSON(w, http.StatusOK, out)
}
