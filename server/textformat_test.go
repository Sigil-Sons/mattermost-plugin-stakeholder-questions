package main

import "testing"

func TestParseQuestionsText(t *testing.T) {
	qs, err := parseQuestionsText(dialogExample)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("want 2 questions, got %d", len(qs))
	}
	q1 := qs[0]
	if q1.Prompt != "Where should the model bridge run?" || q1.HelpMarkdown != "Context: it talks to Blade for inference." || !q1.AllowOther {
		t.Fatalf("q1: %+v", q1)
	}
	if len(q1.Options) != 2 || q1.Options[0].Label != "Hermes LXC" || !q1.Options[0].Recommended || q1.Options[0].Description != "next to the gateway" {
		t.Fatalf("q1 options: %+v", q1.Options)
	}
	if qs[1].Selection != SelectionMulti || qs[1].Prompt != "Which audio features ship in v1?" || len(qs[1].Options) != 3 {
		t.Fatalf("q2: %+v", qs[1])
	}
	if err := normalizeQuestions(qs, 8); err != nil {
		t.Fatalf("parsed output should validate: %v", err)
	}
}

func TestParseQuestionsTextErrors(t *testing.T) {
	if _, err := parseQuestionsText("   "); err == nil {
		t.Error("empty should fail")
	}
	if _, err := parseQuestionsText("A question with no options"); err == nil {
		t.Error("no options should fail")
	}
}

func TestParseRespondents(t *testing.T) {
	got := parseRespondents("@Drew, scott  @drew")
	if len(got) != 2 || got[0] != "drew" || got[1] != "scott" {
		t.Fatalf("%v", got)
	}
}
