package main

import "testing"

func q(id string, opts ...string) Question {
	o := make([]Option, len(opts))
	for i, l := range opts {
		o[i] = Option{Label: l}
	}
	return Question{ID: id, Prompt: "Pick one", Options: o}
}

func TestNormalizeFillsIDsAndDefaults(t *testing.T) {
	qs := []Question{{Prompt: " Which DB? ", Options: []Option{{Label: "Postgres 16"}, {Label: "SQLite"}}}}
	if err := normalizeQuestions(qs, 8); err != nil {
		t.Fatal(err)
	}
	if qs[0].ID != "q1" || qs[0].Selection != SelectionSingle || qs[0].Prompt != "Which DB?" {
		t.Fatalf("defaults not applied: %+v", qs[0])
	}
	if qs[0].Options[0].ID != "postgres-16" || qs[0].Options[1].ID != "sqlite" {
		t.Fatalf("option slugs: %+v", qs[0].Options)
	}
}

func TestNormalizeRejects(t *testing.T) {
	bad := "https://files.catbox.moe/x.png"
	cases := map[string][]Question{
		"empty":        {},
		"too many":     {q("a", "x", "y"), q("b", "x", "y"), q("c", "x", "y")},
		"no prompt":    {{ID: "a", Options: []Option{{Label: "x"}, {Label: "y"}}}},
		"one option":   {q("a", "x")},
		"dup question": {q("a", "x", "y"), q("a", "x", "y")},
		"dup option":   {q("a", "Same", "same")},
		"bad select":   {{ID: "a", Prompt: "p", Selection: "ranked", Options: []Option{{Label: "x"}, {Label: "y"}}}},
		"paste host":   {{ID: "a", Prompt: "p", Options: []Option{{Label: "x", ImageURL: &bad}, {Label: "y"}}}},
	}
	for name, qs := range cases {
		if err := normalizeQuestions(qs, 2); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
	// one option is fine when Other is allowed
	if err := normalizeQuestions([]Question{{ID: "a", Prompt: "p", AllowOther: true, Options: []Option{{Label: "x"}}}}, 2); err != nil {
		t.Errorf("1 + other should pass: %v", err)
	}
}

func TestDefaultMinRespondents(t *testing.T) {
	cases := []struct {
		req, n int
		imp    string
		want   int
	}{
		{0, 2, "normal", 2}, // G3: two stakeholders -> all
		{0, 5, "high", 5},   // high -> all
		{0, 5, "normal", 3}, // lower -> majority
		{1, 2, "high", 1},   // explicit wins
		{9, 3, "low", 3},    // clamp
	}
	for _, c := range cases {
		if got := defaultMinRespondents(c.req, c.n, c.imp); got != c.want {
			t.Errorf("%+v: got %d", c, got)
		}
	}
}

func packWithAnswers(status string) *Pack {
	return &Pack{
		ID: "p", Status: status, AuthorUserID: "bot", RespondentUserIDs: []string{"drew", "scott"},
		Answers: map[string]map[string]Answer{
			"drew":  {"q1": {OptionIDs: []string{"a"}}},
			"scott": {"q1": {OptionIDs: []string{"b"}}},
		},
	}
}

func TestVisibilityRespondentHiddenByDefault(t *testing.T) {
	v := visibleCopy(packWithAnswers(StatusOpen), viewer{UserID: "drew", IsRespondent: true}, true)
	if _, ok := v.Answers["scott"]; ok {
		t.Fatal("others' answers must be hidden by default")
	}
	if _, ok := v.Answers["drew"]; !ok {
		t.Fatal("own answers must be visible")
	}
}

func TestVisibilityNoAutoRevealWhenComplete(t *testing.T) {
	v := visibleCopy(packWithAnswers(StatusComplete), viewer{UserID: "drew", IsRespondent: true}, true)
	if _, ok := v.Answers["scott"]; ok {
		t.Fatal("must not auto-reveal to respondents even when complete (KITT 2026-11-17)")
	}
}

func TestVisibilityOptInRevealsOnlySubmitted(t *testing.T) {
	pk := packWithAnswers(StatusOpen)
	pk.Answers["empty"] = map[string]Answer{}
	v := visibleCopy(pk, viewer{UserID: "drew", IsRespondent: true, ShowOthers: true}, true)
	if _, ok := v.Answers["scott"]; !ok {
		t.Fatal("opt-in should reveal scott's submitted answers")
	}
	if _, ok := v.Answers["empty"]; ok {
		t.Fatal("users with nothing submitted should not appear")
	}
}

func TestVisibilityAuthorSeesAllObserverGated(t *testing.T) {
	if v := visibleCopy(packWithAnswers(StatusOpen), viewer{UserID: "bot", IsAuthor: true}, false); len(v.Answers) != 2 {
		t.Fatal("author (agent read-back) sees all answers")
	}
	if v := visibleCopy(packWithAnswers(StatusOpen), viewer{UserID: "obs"}, true); len(v.Answers) != 0 {
		t.Fatal("observer sees nothing while open")
	}
	if v := visibleCopy(packWithAnswers(StatusComplete), viewer{UserID: "obs"}, true); len(v.Answers) != 2 {
		t.Fatal("observer sees results when complete and allowed")
	}
	if v := visibleCopy(packWithAnswers(StatusComplete), viewer{UserID: "obs"}, false); len(v.Answers) != 0 {
		t.Fatal("observer gated by config")
	}
}

func TestVisibleCopyDoesNotMutate(t *testing.T) {
	pk := packWithAnswers(StatusOpen)
	_ = visibleCopy(pk, viewer{UserID: "drew", IsRespondent: true}, true)
	if len(pk.Answers) != 2 {
		t.Fatal("filter mutated stored pack")
	}
}
