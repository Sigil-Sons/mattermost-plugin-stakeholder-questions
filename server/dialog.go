package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

const dialogExample = `Where should the model bridge run?
Context: it talks to Blade for inference.
- Hermes LXC (recommended) :: next to the gateway
- Blade
- Other

[multi] Which audio features ship in v1?
- TTS narration
- Ambient music
- SFX`

func (p *Plugin) openCreateDialog(triggerID, channelID string) *model.AppError {
	return p.API.OpenInteractiveDialog(model.OpenDialogRequest{
		TriggerId: triggerID,
		URL:       "/plugins/com.mysticspark.stakeholder-questions/api/v1/dialog/create",
		Dialog: model.Dialog{
			CallbackId:  "ssq-create",
			Title:       "New stakeholder pack",
			SubmitLabel: "Post pack",
			State:       channelID,
			Elements: []model.DialogElement{
				{DisplayName: "Title", Name: "title", Type: "text", MaxLength: 120, Placeholder: "StoryBard deployment topology"},
				{DisplayName: "Who needs to answer", Name: "respondents", Type: "text", Placeholder: "@drew @scott",
					HelpText: "Usernames in this channel."},
				{DisplayName: "Importance", Name: "importance", Type: "select", Default: "normal", Optional: true,
					HelpText: "High = everyone must answer. Normal/low with 3+ people = majority.",
					Options:  []*model.PostActionOptions{{Text: "Normal", Value: "normal"}, {Text: "High", Value: "high"}, {Text: "Low", Value: "low"}}},
				{DisplayName: "Questions", Name: "questions", Type: "textarea", MaxLength: 3000, Placeholder: dialogExample,
					HelpText: QuestionFormatHelp},
				{DisplayName: "Summary", Name: "summary", Type: "text", Optional: true, MaxLength: 200},
			},
		},
	})
}

func subStr(v interface{}) string {
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// handleDialogCreate receives the /ssq new form. Errors go back inline on the form.
func (p *Plugin) handleDialogCreate(w http.ResponseWriter, r *http.Request, userID string) {
	var sub model.SubmitDialogRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&sub); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}
	if sub.Cancelled {
		writeJSON(w, http.StatusOK, map[string]string{})
		return
	}
	channelID := sub.State
	if channelID == "" {
		channelID = sub.ChannelId
	}
	qs, err := parseQuestionsText(subStr(sub.Submission["questions"]))
	if err != nil {
		writeJSON(w, http.StatusOK, model.SubmitDialogResponse{Errors: map[string]string{"questions": err.Error()}})
		return
	}
	req := CreateRequest{Title: subStr(sub.Submission["title"]), Summary: subStr(sub.Submission["summary"]), ChannelID: channelID,
		Respondents: parseRespondents(subStr(sub.Submission["respondents"])), Importance: subStr(sub.Submission["importance"]), Questions: qs}
	if len(req.Respondents) == 0 {
		writeJSON(w, http.StatusOK, model.SubmitDialogResponse{Errors: map[string]string{"respondents": "Add at least one @username."}})
		return
	}
	if _, cErr := p.createPack(userID, req); cErr != nil {
		field := ""
		switch {
		case strings.Contains(cErr.msg, "member") || strings.Contains(cErr.msg, "unknown user"):
			field = "respondents"
		case strings.HasPrefix(cErr.msg, "question") || strings.Contains(cErr.msg, "questions"):
			field = "questions"
		case strings.Contains(cErr.msg, "title"):
			field = "title"
		}
		if field != "" {
			writeJSON(w, http.StatusOK, model.SubmitDialogResponse{Errors: map[string]string{field: cErr.msg}})
		} else {
			writeJSON(w, http.StatusOK, model.SubmitDialogResponse{Error: cErr.msg})
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{})
}
