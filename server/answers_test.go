package main

import (
	"errors"
	"strings"
	"testing"
)

func testPack() *Pack {
	qs := []Question{
		{ID: "q1", Prompt: "Where?", Selection: SelectionSingle, AllowOther: true,
			Options: []Option{{ID: "lxc", Label: "Hermes LXC"}, {ID: "blade", Label: "Blade"}}},
		{ID: "q2", Prompt: "Audio?", Selection: SelectionMulti,
			Options: []Option{{ID: "tts", Label: "TTS"}, {ID: "music", Label: "Music"}, {ID: "sfx", Label: "SFX"}}},
	}
	return &Pack{ID: "p", Title: "T", Status: StatusOpen, Questions: qs, AllowReviseUntilLock: true,
		RespondentUserIDs: []string{"d", "s"}, RespondentUsernames: []string{"drew", "scott"}, MinRespondents: 2,
		Answers: map[string]map[string]Answer{}, CompletedUserIDs: []string{}}
}

func str(s string) *string { return &s }

func TestBuildAnswerRules(t *testing.T) {
	pk := testPack()
	ok := []AnswerRequest{
		{QuestionID: "q1", OptionIDs: []string{"lxc"}},
		{QuestionID: "q1", OtherText: str("both")},
		{QuestionID: "q2", OptionIDs: []string{"tts", "sfx", "tts"}},
	}
	for _, r := range ok {
		if _, err := buildAnswer(pk, r, 1); err != nil {
			t.Errorf("%+v: %v", r, err)
		}
	}
	a, _ := buildAnswer(pk, AnswerRequest{QuestionID: "q2", OptionIDs: []string{"tts", "sfx", "tts"}}, 1)
	if len(a.OptionIDs) != 2 {
		t.Errorf("multi should dedupe: %v", a.OptionIDs)
	}
	bad := []AnswerRequest{
		{QuestionID: "nope", OptionIDs: []string{"lxc"}},
		{QuestionID: "q1", OptionIDs: []string{"lxc", "blade"}},
		{QuestionID: "q1", OptionIDs: []string{"zzz"}},
		{QuestionID: "q1"},
		{QuestionID: "q1", OptionIDs: []string{"lxc"}, OtherText: str("also")},
		{QuestionID: "q2", OtherText: str("no other here")},
		{QuestionID: "q1", Skip: true},
		{QuestionID: "q2", OptionIDs: []string{"tts"}, ImageFileIDs: []string{"1", "2", "3", "4", "5", "6"}},
	}
	for _, r := range bad {
		if _, err := buildAnswer(pk, r, 1); err == nil {
			t.Errorf("%+v: expected error", r)
		}
	}
	pk.AllowSkip = true
	if a, err := buildAnswer(pk, AnswerRequest{QuestionID: "q1", Skip: true}, 1); err != nil || !a.Skipped {
		t.Errorf("skip allowed: %v %+v", err, a)
	}
}

func answer(t *testing.T, pk *Pack, u, q string, opts ...string) applyResult {
	t.Helper()
	a, err := buildAnswer(pk, AnswerRequest{QuestionID: q, OptionIDs: opts}, 5)
	if err != nil {
		t.Fatal(err)
	}
	res, err := applyAnswer(pk, u, q, a, 5)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestCompletionQuorumAndLock(t *testing.T) {
	pk := testPack()
	if r := answer(t, pk, "d", "q1", "lxc"); r.UserCompleted {
		t.Fatal("not complete after 1 of 2")
	}
	if r := answer(t, pk, "d", "q2", "tts", "sfx"); !r.UserCompleted || r.PackCompleted {
		t.Fatalf("drew complete, pack still open: %+v", r)
	}
	// A7 revise before lock
	answer(t, pk, "d", "q1", "blade")
	if got := pk.Answers["d"]["q1"].OptionIDs[0]; got != "blade" {
		t.Fatalf("revise: %s", got)
	}
	answer(t, pk, "s", "q1", "lxc")
	if r := answer(t, pk, "s", "q2", "music"); !r.PackCompleted || pk.Status != StatusComplete || pk.LockedAt == nil {
		t.Fatalf("quorum should complete+lock: %+v %s", r, pk.Status)
	}
	a, _ := buildAnswer(pk, AnswerRequest{QuestionID: "q1", OptionIDs: []string{"lxc"}}, 9)
	if _, err := applyAnswer(pk, "d", "q1", a, 9); !errors.Is(err, errLocked) {
		t.Fatalf("locked pack must reject: %v", err)
	}
}

func TestNonRespondentRejected(t *testing.T) {
	pk := testPack()
	a, _ := buildAnswer(pk, AnswerRequest{QuestionID: "q1", OptionIDs: []string{"lxc"}}, 1)
	if _, err := applyAnswer(pk, "observer", "q1", a, 1); !errors.Is(err, errNotRespondent) {
		t.Fatal(err)
	}
}

func TestNofMQuorum(t *testing.T) {
	pk := testPack()
	pk.MinRespondents = 1
	answer(t, pk, "s", "q1", "lxc")
	if r := answer(t, pk, "s", "q2", "tts"); !r.PackCompleted {
		t.Fatal("1-of-2 quorum should complete on first finisher")
	}
}

func TestSummaryMarkdown(t *testing.T) {
	pk := testPack()
	answer(t, pk, "d", "q1", "lxc")
	answer(t, pk, "d", "q2", "tts", "sfx")
	answer(t, pk, "s", "q1", "lxc")
	answer(t, pk, "s", "q2", "tts")
	md := summaryMarkdown(pk)
	for _, want := range []string{"# T", "## Decisions", "**Decision:** Hermes LXC (all 2 agree)",
		"**Decision:** TTS (all 2 agree)", "- SFX: 1 (@drew)", "Completed: 2/2"} {
		if !strings.Contains(md, want) {
			t.Errorf("summary missing %q\n%s", want, md)
		}
	}
	pk2 := testPack()
	answer(t, pk2, "d", "q1", "lxc")
	o := "split by load"
	pk2.Answers["s"] = map[string]Answer{"q1": {OtherText: &o}}
	md2 := summaryMarkdown(pk2)
	if !strings.Contains(md2, "**Split:** most picked Hermes LXC (1/2)") || !strings.Contains(md2, "Other, @scott: split by load") {
		t.Errorf("split summary wrong:\n%s", md2)
	}
}

func TestDueActions(t *testing.T) {
	pk := testPack()
	pk.CreatedAt = 0
	if exp, n := dueActions(pk, nudgeAfterMs-1); exp || len(n) != 0 {
		t.Fatal("nothing due before 24h")
	}
	pk.CompletedUserIDs = []string{"d"}
	if _, n := dueActions(pk, nudgeAfterMs); len(n) != 1 || n[0] != "s" {
		t.Fatalf("nudge scott only: %v", n)
	}
	pk.NudgedUserIDs = []string{"s"}
	if _, n := dueActions(pk, nudgeAfterMs*2); len(n) != 0 {
		t.Fatal("nudge once")
	}
	c := int64(100)
	pk.ClosesAt = &c
	if exp, _ := dueActions(pk, 100); !exp {
		t.Fatal("should expire at closes_at")
	}
	pk.Status = StatusComplete
	if exp, _ := dueActions(pk, 100); exp {
		t.Fatal("closed packs never expire")
	}
}

func TestCloseMessageMentionsBotAuthorOnly(t *testing.T) {
	pk := testPack()
	pk.AuthorBot = true
	if m := closeMessage(pk, "✅ Pack complete:", "sigil"); !strings.HasPrefix(m, "@sigil ✅ Pack complete: **T**") || !strings.Contains(m, "## Decisions") {
		t.Fatalf("bot author should be mentioned first: %q", m[:60])
	}
	pk.AuthorBot = false
	if m := closeMessage(pk, "✅ Pack complete:", "drew"); strings.HasPrefix(m, "@") {
		t.Fatal("human authors are not pinged")
	}
}

func TestFallbackMessageListsQuestionsAndLink(t *testing.T) {
	pk := testPack()
	pk.Questions[0].Options[0].Recommended = true
	m := fallbackMessage(pk, []string{"drew", "scott"}, "https://mm.test/plugins/x/answer/p")
	for _, want := range []string{"@drew @scott, 2 question(s)", "1. Where? _(pick one)_: Hermes LXC (recommended) · Blade · Other",
		"2. Audio? _(pick any)_: TTS · Music · SFX", "📱 [Answer in browser](https://mm.test/plugins/x/answer/p)"} {
		if !strings.Contains(m, want) {
			t.Errorf("missing %q in\n%s", want, m)
		}
	}
}
