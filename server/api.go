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
	parts := strings.Split(strings.TrimPrefix(path, "/api/v1/"), "/")
	switch {
	case path == "/api/v1/packs" && r.Method == http.MethodPost:
		p.handleCreate(w, r, userID)
	case path == "/api/v1/me/pending" && r.Method == http.MethodGet:
		p.handlePending(w, r, userID)
	case len(parts) == 2 && parts[0] == "packs" && r.Method == http.MethodGet:
		p.handleGet(w, r, userID, parts[1])
	case len(parts) == 3 && parts[0] == "packs" && parts[2] == "answers" && r.Method == http.MethodPost:
		p.handleAnswer(w, r, userID, parts[1])
	case len(parts) == 3 && parts[0] == "packs" && parts[2] == "lock" && r.Method == http.MethodPost:
		p.handleLock(w, r, userID, parts[1])
	case len(parts) == 3 && parts[0] == "packs" && parts[2] == "summary.md" && r.Method == http.MethodGet:
		p.handleSummary(w, r, userID, parts[1])
	case len(parts) == 4 && parts[0] == "packs" && parts[2] == "files" && r.Method == http.MethodGet:
		p.handleFile(w, r, userID, parts[1], parts[3])
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
	for _, q := range req.Questions {
		fids := append([]string{}, q.HelpFileIDs...)
		for _, o := range q.Options {
			if o.ImageFileID != nil && *o.ImageFileID != "" {
				fids = append(fids, *o.ImageFileID)
			}
		}
		if err := p.checkOwnFiles(fids, userID); err != nil {
			writeErr(w, http.StatusBadRequest, "question "+q.ID+": "+err.Error())
			return
		}
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
		RespondentUsernames: names,
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
	post.SetProps(cardProps(pk))
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
func cardProps(pk *Pack) model.StringInterface {
	done := []string{}
	for _, u := range pk.CompletedUserIDs {
		done = append(done, pk.username(u))
	}
	return model.StringInterface{
		"ssq_pack_id":         pk.ID,
		"ssq_title":           pk.Title,
		"ssq_summary":         pk.Summary,
		"ssq_status":          pk.Status,
		"ssq_question_count":  len(pk.Questions),
		"ssq_min_respondents": pk.MinRespondents,
		"ssq_respondents":     pk.RespondentUsernames,
		"ssq_respondent_ids":  pk.RespondentUserIDs,
		"ssq_completed":       done,
		"ssq_completed_ids":   pk.CompletedUserIDs,
		"ssq_author_id":       pk.AuthorUserID,
		"ssq_created_at":      pk.CreatedAt,
		"ssq_updated_at":      model.GetMillis(),
	}
}

func (p *Plugin) checkOwnFiles(ids []string, userID string) error {
	for _, id := range ids {
		info, appErr := p.API.GetFileInfo(id)
		if appErr != nil || info == nil {
			return fmt.Errorf("file %s not found", id)
		}
		if info.CreatorId != userID {
			return fmt.Errorf("file %s was not uploaded by you", id)
		}
		if !strings.HasPrefix(info.MimeType, "image/") {
			return fmt.Errorf("file %s is not an image", id)
		}
	}
	return nil
}

// updateCard re-renders the channel card from pack state. Caller holds kvLock.
func (p *Plugin) updateCard(pk *Pack) {
	post, appErr := p.API.GetPost(pk.PostID)
	if appErr != nil || post == nil {
		return
	}
	for k, v := range cardProps(pk) {
		post.AddProp(k, v)
	}
	if _, appErr := p.API.UpdatePost(post); appErr != nil {
		p.API.LogWarn("ssq: card update failed", "pack", pk.ID, "err", appErr.Error())
	}
}

func (p *Plugin) clearPending(pk *Pack, only ...string) {
	users := pk.RespondentUserIDs
	if len(only) > 0 {
		users = only
	}
	for _, u := range users {
		_ = p.indexRemove(userPendingKey(u), pk.ID)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n\n…(truncated; full log via summary.md)"
}

// postInThread replies under the pack card as the plugin bot.
func (p *Plugin) postInThread(pk *Pack, msg string) {
	if p.botID == "" {
		return
	}
	if _, appErr := p.API.CreatePost(&model.Post{UserId: p.botID, ChannelId: pk.ChannelID, RootId: pk.PostID, Message: msg}); appErr != nil {
		p.API.LogWarn("ssq: thread reply failed", "pack", pk.ID, "err", appErr.Error())
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

func (p *Plugin) handleAnswer(w http.ResponseWriter, r *http.Request, userID, id string) {
	var req AnswerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad json: "+err.Error())
		return
	}
	p.kvLock.Lock()
	defer p.kvLock.Unlock()
	pk, err := p.getPack(id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "load failed")
		return
	}
	if pk == nil {
		writeErr(w, http.StatusNotFound, "pack not found")
		return
	}
	if !contains(pk.RespondentUserIDs, userID) {
		writeErr(w, http.StatusForbidden, errNotRespondent.Error())
		return
	}
	now := model.GetMillis()
	ans, err := buildAnswer(pk, req, now)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := p.checkOwnFiles(ans.ImageFileIDs, userID); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	res, err := applyAnswer(pk, userID, req.QuestionID, ans, now)
	switch {
	case err == errLocked:
		writeErr(w, http.StatusConflict, err.Error())
		return
	case err == errNotRespondent:
		writeErr(w, http.StatusForbidden, err.Error())
		return
	case err != nil:
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := p.savePack(pk); err != nil {
		writeErr(w, http.StatusInternalServerError, "store failed")
		return
	}
	if res.UserCompleted {
		p.clearPending(pk, userID)
	}
	if res.UserCompleted || res.PackCompleted {
		p.updateCard(pk)
	}
	if res.PackCompleted {
		p.clearPending(pk)
		p.postInThread(pk, "✅ **Pack complete: "+pk.Title+"**\n\n"+truncate(summaryMarkdown(pk), 12000))
	}
	v, _ := p.viewerFor(pk, userID, r)
	writeJSON(w, http.StatusOK, visibleCopy(pk, v, p.config().AllowObserverResultsOnComplete))
}

func (p *Plugin) handleLock(w http.ResponseWriter, r *http.Request, userID, id string) {
	p.kvLock.Lock()
	defer p.kvLock.Unlock()
	pk, err := p.getPack(id)
	if err != nil || pk == nil {
		writeErr(w, http.StatusNotFound, "pack not found")
		return
	}
	v, _ := p.viewerFor(pk, userID, r)
	if !v.IsAuthor && !v.IsAdmin {
		writeErr(w, http.StatusForbidden, "only the author or an admin can lock a pack")
		return
	}
	if pk.Status == StatusOpen {
		now := model.GetMillis()
		pk.Status = StatusLocked
		pk.LockedAt = &now
		if err := p.savePack(pk); err != nil {
			writeErr(w, http.StatusInternalServerError, "store failed")
			return
		}
		p.clearPending(pk)
		p.updateCard(pk)
		p.postInThread(pk, "🔒 **Pack locked: "+pk.Title+"**\n\n"+truncate(summaryMarkdown(pk), 12000))
	}
	writeJSON(w, http.StatusOK, visibleCopy(pk, v, p.config().AllowObserverResultsOnComplete))
}

func (p *Plugin) handleSummary(w http.ResponseWriter, r *http.Request, userID, id string) {
	pk, err := p.getPack(id)
	if err != nil || pk == nil {
		writeErr(w, http.StatusNotFound, "pack not found")
		return
	}
	v, ok := p.viewerFor(pk, userID, r)
	if !ok {
		writeErr(w, http.StatusNotFound, "pack not found")
		return
	}
	v.ShowOthers = true // asking for the decision log is an explicit opt-in
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="summary.md"`)
	_, _ = w.Write([]byte(summaryMarkdown(visibleCopy(pk, v, p.config().AllowObserverResultsOnComplete))))
}

// handleFile serves images referenced by the pack, filtered by the same visibility rules as answers.
func (p *Plugin) handleFile(w http.ResponseWriter, r *http.Request, userID, id, fileID string) {
	pk, err := p.getPack(id)
	if err != nil || pk == nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	v, ok := p.viewerFor(pk, userID, r)
	if !ok {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	vis := visibleCopy(pk, v, p.config().AllowObserverResultsOnComplete)
	allowed := false
	for _, q := range vis.Questions {
		if contains(q.HelpFileIDs, fileID) {
			allowed = true
		}
		for _, o := range q.Options {
			if o.ImageFileID != nil && *o.ImageFileID == fileID {
				allowed = true
			}
		}
	}
	for _, byQ := range vis.Answers {
		for _, a := range byQ {
			if contains(a.ImageFileIDs, fileID) {
				allowed = true
			}
		}
	}
	if !allowed {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	info, appErr := p.API.GetFileInfo(fileID)
	if appErr != nil || !strings.HasPrefix(info.MimeType, "image/") {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	data, appErr := p.API.GetFile(fileID)
	if appErr != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	w.Header().Set("Content-Type", info.MimeType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(data)
}
