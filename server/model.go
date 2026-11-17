package main

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	PostType = "custom_ssq_pack"

	StatusOpen     = "open"
	StatusComplete = "complete"
	StatusLocked   = "locked"
	StatusExpired  = "expired"

	SelectionSingle = "single"
	SelectionMulti  = "multi"

	VisibilityOthersOptIn = "others_opt_in"
)

type Option struct {
	ID          string  `json:"id"`
	Label       string  `json:"label"`
	Description string  `json:"description,omitempty"`
	Recommended bool    `json:"recommended,omitempty"`
	ImageFileID *string `json:"image_file_id"`
	ImageURL    *string `json:"image_url"`
}

type Question struct {
	ID           string   `json:"id"`
	Prompt       string   `json:"prompt"`
	HelpMarkdown string   `json:"help_markdown,omitempty"`
	HelpFileIDs  []string `json:"help_file_ids"`
	Selection    string   `json:"selection"`
	AllowOther   bool     `json:"allow_other"`
	Options      []Option `json:"options"`
}

type Answer struct {
	OptionIDs    []string `json:"option_ids"`
	OtherText    *string  `json:"other_text"`
	ImageFileIDs []string `json:"image_file_ids"`
	UpdatedAt    int64    `json:"updated_at"`
}

type Pack struct {
	ID                   string                       `json:"id"`
	TeamID               string                       `json:"team_id"`
	ChannelID            string                       `json:"channel_id"`
	PostID               string                       `json:"post_id"`
	Title                string                       `json:"title"`
	Summary              string                       `json:"summary,omitempty"`
	AuthorUserID         string                       `json:"author_user_id"`
	AuthorBot            bool                         `json:"author_bot"`
	RespondentUserIDs    []string                     `json:"respondent_user_ids"`
	MinRespondents       int                          `json:"min_respondents"`
	Importance           string                       `json:"importance"`
	Visibility           string                       `json:"visibility"`
	AllowSkip            bool                         `json:"allow_skip"`
	AllowReviseUntilLock bool                         `json:"allow_revise_until_lock"`
	Status               string                       `json:"status"`
	CreatedAt            int64                        `json:"created_at"`
	ClosesAt             *int64                       `json:"closes_at"`
	LockedAt             *int64                       `json:"locked_at"`
	Questions            []Question                   `json:"questions"`
	Answers              map[string]map[string]Answer `json:"answers"`
	CompletedUserIDs     []string                     `json:"completed_user_ids"`
}

// CreateRequest is the bot/agent create body (SPEC §6, §10).
type CreateRequest struct {
	Title          string     `json:"title"`
	Summary        string     `json:"summary"`
	ChannelID      string     `json:"channel_id"`
	Respondents    []string   `json:"respondents"` // usernames (with or without @) or user ids
	Questions      []Question `json:"questions"`
	ClosesAt       *int64     `json:"closes_at"`
	MinRespondents int        `json:"min_respondents"`
	Importance     string     `json:"importance"`
	AllowSkip      bool       `json:"allow_skip"`
}

var slugRe = regexp.MustCompile(`[^a-z0-9_-]+`)

func slug(s string) string {
	s = slugRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
	}
	return s
}

// normalizeQuestions validates and fills ids/defaults in place. maxQ is the configured cap.
func normalizeQuestions(qs []Question, maxQ int) error {
	if len(qs) == 0 {
		return fmt.Errorf("pack needs at least one question")
	}
	if len(qs) > maxQ {
		return fmt.Errorf("pack has %d questions; max is %d", len(qs), maxQ)
	}
	seenQ := map[string]bool{}
	for i := range qs {
		q := &qs[i]
		q.Prompt = strings.TrimSpace(q.Prompt)
		if q.Prompt == "" {
			return fmt.Errorf("question %d: prompt is required", i+1)
		}
		if q.ID == "" {
			q.ID = fmt.Sprintf("q%d", i+1)
		}
		if seenQ[q.ID] {
			return fmt.Errorf("question %d: duplicate id %q", i+1, q.ID)
		}
		seenQ[q.ID] = true
		if q.Selection == "" {
			q.Selection = SelectionSingle
		}
		if q.Selection != SelectionSingle && q.Selection != SelectionMulti {
			return fmt.Errorf("question %s: selection must be single or multi", q.ID)
		}
		if q.HelpFileIDs == nil {
			q.HelpFileIDs = []string{}
		}
		if len(q.Options) < 2 && !(q.AllowOther && len(q.Options) >= 1) {
			return fmt.Errorf("question %s: needs at least 2 options (or 1 + allow_other)", q.ID)
		}
		seenO := map[string]bool{}
		for j := range q.Options {
			o := &q.Options[j]
			o.Label = strings.TrimSpace(o.Label)
			if o.Label == "" {
				return fmt.Errorf("question %s option %d: label is required", q.ID, j+1)
			}
			if o.ID == "" {
				o.ID = slug(o.Label)
				if o.ID == "" {
					o.ID = fmt.Sprintf("o%d", j+1)
				}
			}
			if seenO[o.ID] {
				return fmt.Errorf("question %s: duplicate option id %q", q.ID, o.ID)
			}
			seenO[o.ID] = true
			if o.ImageURL != nil && !allowedImageURL(*o.ImageURL) {
				return fmt.Errorf("question %s option %s: image_url must be https on a house host (Keep/MM), not a public paste host", q.ID, o.ID)
			}
		}
	}
	return nil
}

// allowedImageURL: https only, no public paste hosts (SPEC §7.2). Keep / MM files are the intended sources.
func allowedImageURL(u string) bool {
	l := strings.ToLower(strings.TrimSpace(u))
	if !strings.HasPrefix(l, "https://") {
		return false
	}
	for _, bad := range []string{"catbox.moe", "imgur.com", "pastebin", "litter.catbox", "0x0.st", "transfer.sh"} {
		if strings.Contains(l, bad) {
			return false
		}
	}
	return true
}

// defaultMinRespondents implements G3: high importance -> everyone; two or fewer -> everyone; else majority.
func defaultMinRespondents(requested, n int, importance string) int {
	if n == 0 {
		return 0
	}
	if requested > 0 {
		if requested > n {
			return n
		}
		return requested
	}
	if importance == "high" || n <= 2 {
		return n
	}
	return (n + 1) / 2
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// Viewer roles for the visibility filter.
type viewer struct {
	UserID       string
	IsAuthor     bool
	IsAdmin      bool
	IsRespondent bool
	ShowOthers   bool // explicit opt-in (?show_others=1)
}

// visibleCopy returns the pack as this viewer may see it (SPEC §4.3, G4; KITT 2026-11-17):
//   - author / admin: all answers (agent read-back, §3.4)
//   - respondent: own answers always; others' submitted answers only with explicit opt-in; never auto-revealed
//   - observer: no answers unless pack complete and observers allowed
func visibleCopy(p *Pack, v viewer, observersOnComplete bool) *Pack {
	out := *p
	out.Answers = map[string]map[string]Answer{}
	switch {
	case v.IsAuthor || v.IsAdmin:
		for u, a := range p.Answers {
			out.Answers[u] = a
		}
	case v.IsRespondent:
		if a, ok := p.Answers[v.UserID]; ok {
			out.Answers[v.UserID] = a
		}
		if v.ShowOthers {
			for u, a := range p.Answers {
				if len(a) > 0 {
					out.Answers[u] = a
				}
			}
		}
	default:
		if p.Status == StatusComplete && observersOnComplete {
			for u, a := range p.Answers {
				out.Answers[u] = a
			}
		}
	}
	return &out
}
