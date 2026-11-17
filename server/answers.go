package main

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

var (
	errLocked        = errors.New("pack is closed; answers can no longer change")
	errNotRespondent = errors.New("only respondents can answer this pack")
)

const (
	maxOtherText    = 2000
	maxAnswerImages = 5
	nudgeAfterMs    = int64(24 * 60 * 60 * 1000)
)

// AnswerRequest is one question's answer (POST /packs/{id}/answers).
type AnswerRequest struct {
	QuestionID   string   `json:"question_id"`
	OptionIDs    []string `json:"option_ids"`
	OtherText    *string  `json:"other_text"`
	ImageFileIDs []string `json:"image_file_ids"`
	Skip         bool     `json:"skip"`
}

func (pk *Pack) question(id string) *Question {
	for i := range pk.Questions {
		if pk.Questions[i].ID == id {
			return &pk.Questions[i]
		}
	}
	return nil
}

func dedupe(xs []string) []string {
	out := []string{}
	for _, x := range xs {
		x = strings.TrimSpace(x)
		if x != "" && !contains(out, x) {
			out = append(out, x)
		}
	}
	return out
}

// buildAnswer validates an answer against its question and the pack rules.
func buildAnswer(pk *Pack, req AnswerRequest, now int64) (Answer, error) {
	q := pk.question(req.QuestionID)
	if q == nil {
		return Answer{}, fmt.Errorf("unknown question %q", req.QuestionID)
	}
	if req.Skip {
		if !pk.AllowSkip {
			return Answer{}, fmt.Errorf("this pack does not allow skipping")
		}
		return Answer{OptionIDs: []string{}, ImageFileIDs: []string{}, UpdatedAt: now, Skipped: true}, nil
	}
	ids := dedupe(req.OptionIDs)
	for _, id := range ids {
		found := false
		for _, o := range q.Options {
			if o.ID == id {
				found = true
				break
			}
		}
		if !found {
			return Answer{}, fmt.Errorf("question %s has no option %q", q.ID, id)
		}
	}
	if q.Selection == SelectionSingle && len(ids) > 1 {
		return Answer{}, fmt.Errorf("question %s takes one choice", q.ID)
	}
	other := ""
	if req.OtherText != nil {
		other = strings.TrimSpace(*req.OtherText)
	}
	if other != "" && !q.AllowOther {
		return Answer{}, fmt.Errorf("question %s does not allow Other", q.ID)
	}
	if len(other) > maxOtherText {
		return Answer{}, fmt.Errorf("Other text is too long (max %d characters)", maxOtherText)
	}
	if q.Selection == SelectionSingle && len(ids) == 1 && other != "" {
		return Answer{}, fmt.Errorf("question %s: pick one option or Other, not both", q.ID)
	}
	if len(ids) == 0 && other == "" {
		return Answer{}, fmt.Errorf("question %s: choose an option", q.ID)
	}
	imgs := dedupe(req.ImageFileIDs)
	if len(imgs) > maxAnswerImages {
		return Answer{}, fmt.Errorf("at most %d images per answer", maxAnswerImages)
	}
	a := Answer{OptionIDs: ids, ImageFileIDs: imgs, UpdatedAt: now}
	if other != "" {
		a.OtherText = &other
	}
	return a, nil
}

type applyResult struct {
	UserCompleted bool
	PackCompleted bool
}

// applyAnswer stores an answer, tracks per-user completion and closes the pack when quorum is met (G8).
func applyAnswer(pk *Pack, userID, questionID string, a Answer, now int64) (applyResult, error) {
	res := applyResult{}
	if pk.Status != StatusOpen {
		return res, errLocked
	}
	if !contains(pk.RespondentUserIDs, userID) {
		return res, errNotRespondent
	}
	if pk.Answers == nil {
		pk.Answers = map[string]map[string]Answer{}
	}
	if pk.Answers[userID] == nil {
		pk.Answers[userID] = map[string]Answer{}
	}
	if _, had := pk.Answers[userID][questionID]; had && !pk.AllowReviseUntilLock {
		return res, fmt.Errorf("answers in this pack cannot be revised")
	}
	pk.Answers[userID][questionID] = a
	if !contains(pk.CompletedUserIDs, userID) && len(pk.Answers[userID]) >= len(pk.Questions) {
		pk.CompletedUserIDs = append(pk.CompletedUserIDs, userID)
		res.UserCompleted = true
	}
	if pk.MinRespondents > 0 && len(pk.CompletedUserIDs) >= pk.MinRespondents {
		pk.Status = StatusComplete
		pk.LockedAt = &now
		res.PackCompleted = true
	}
	return res, nil
}

// dueActions: expire past closes_at; otherwise nudge pending respondents once after 24h (SPEC §8).
func dueActions(pk *Pack, now int64) (expire bool, nudge []string) {
	if pk.Status != StatusOpen {
		return false, nil
	}
	if pk.ClosesAt != nil && now >= *pk.ClosesAt {
		return true, nil
	}
	if now-pk.CreatedAt < nudgeAfterMs {
		return false, nil
	}
	for _, u := range pk.RespondentUserIDs {
		if !contains(pk.CompletedUserIDs, u) && !contains(pk.NudgedUserIDs, u) {
			nudge = append(nudge, u)
		}
	}
	return false, nudge
}

func (pk *Pack) username(uid string) string {
	for i, id := range pk.RespondentUserIDs {
		if id == uid && i < len(pk.RespondentUsernames) {
			return pk.RespondentUsernames[i]
		}
	}
	return uid
}

// summaryMarkdown is the decision log (A4). It reflects whatever answers are in pk (callers pass a visibility-filtered copy).
func summaryMarkdown(pk *Pack) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", pk.Title)
	if pk.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", pk.Summary)
	}
	names := make([]string, len(pk.RespondentUserIDs))
	for i, u := range pk.RespondentUserIDs {
		names[i] = "@" + pk.username(u)
	}
	fmt.Fprintf(&b, "- Status: %s\n- Respondents: %s\n- Completed: %d/%d (quorum %d)\n\n## Decisions\n\n",
		pk.Status, strings.Join(names, ", "), len(pk.CompletedUserIDs), len(pk.RespondentUserIDs), pk.MinRespondents)
	for i, q := range pk.Questions {
		picks := map[string][]string{}
		var others []string
		answered, skipped := 0, 0
		for _, u := range pk.RespondentUserIDs {
			a, ok := pk.Answers[u][q.ID]
			if !ok {
				continue
			}
			if a.Skipped {
				skipped++
				continue
			}
			answered++
			for _, o := range a.OptionIDs {
				picks[o] = append(picks[o], "@"+pk.username(u))
			}
			if a.OtherText != nil {
				others = append(others, fmt.Sprintf("@%s: %s", pk.username(u), *a.OtherText))
			}
		}
		fmt.Fprintf(&b, "### %d. %s\n\n", i+1, q.Prompt)
		var agreed []string
		best, bestN := []string{}, 0
		for _, o := range q.Options {
			n := len(picks[o.ID])
			if answered > 0 && n == answered {
				agreed = append(agreed, o.Label)
			}
			if n > bestN {
				best, bestN = []string{o.Label}, n
			} else if n == bestN && n > 0 {
				best = append(best, o.Label)
			}
		}
		switch {
		case answered == 0:
			b.WriteString("_No answers yet._\n\n")
		case len(agreed) > 0 && len(others) == 0:
			fmt.Fprintf(&b, "**Decision:** %s (all %d agree)\n\n", strings.Join(agreed, ", "), answered)
		case bestN > 0:
			fmt.Fprintf(&b, "**Split:** most picked %s (%d/%d)\n\n", strings.Join(best, ", "), bestN, answered)
		default:
			b.WriteString("**Split:** answered with Other only\n\n")
		}
		for _, o := range q.Options {
			who := picks[o.ID]
			sort.Strings(who)
			line := fmt.Sprintf("- %s: %d", o.Label, len(who))
			if len(who) > 0 {
				line += " (" + strings.Join(who, ", ") + ")"
			}
			b.WriteString(line + "\n")
		}
		for _, o := range others {
			b.WriteString("- Other, " + o + "\n")
		}
		if skipped > 0 {
			fmt.Fprintf(&b, "- Skipped: %d\n", skipped)
		}
		b.WriteString("\n")
	}
	return b.String()
}
