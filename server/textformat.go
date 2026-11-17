package main

import (
	"fmt"
	"regexp"
	"strings"
)

// QuestionFormatHelp explains the plain-text format used by /ssq new.
const QuestionFormatHelp = `One question per block, blank line between blocks. First line = the question; "[multi]" in front allows several picks.
Lines starting with "-" are options: add "(recommended)" to flag one, "Other" adds a free-text choice, "Label :: description" adds detail.
Any other lines are context (markdown).`

var (
	multiRe = regexp.MustCompile(`(?i)^\[multi\]\s*`)
	recRe   = regexp.MustCompile(`(?i)\s*\((recommended|rec)\)\s*`)
)

// parseQuestionsText turns the human format into Questions (validated later by normalizeQuestions).
func parseQuestionsText(s string) ([]Question, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\r\n", "\n")
	if s == "" {
		return nil, fmt.Errorf("add at least one question")
	}
	var out []Question
	for bi, block := range regexp.MustCompile(`\n\s*\n`).Split(s, -1) {
		var q Question
		var help []string
		for _, raw := range strings.Split(block, "\n") {
			line := strings.TrimSpace(raw)
			if line == "" {
				continue
			}
			if q.Prompt == "" {
				if multiRe.MatchString(line) {
					q.Selection = SelectionMulti
					line = multiRe.ReplaceAllString(line, "")
				}
				q.Prompt = line
				continue
			}
			if strings.HasPrefix(line, "-") || strings.HasPrefix(line, "*") {
				label := strings.TrimSpace(line[1:])
				if strings.EqualFold(label, "other") || strings.EqualFold(strings.TrimRight(label, ".…"), "other") {
					q.AllowOther = true
					continue
				}
				o := Option{}
				if recRe.MatchString(label) {
					o.Recommended = true
					label = strings.TrimSpace(recRe.ReplaceAllString(label, " "))
				}
				if i := strings.Index(label, "::"); i >= 0 {
					o.Description = strings.TrimSpace(label[i+2:])
					label = strings.TrimSpace(label[:i])
				}
				o.Label = label
				q.Options = append(q.Options, o)
				continue
			}
			help = append(help, line)
		}
		if q.Prompt == "" {
			continue
		}
		if len(q.Options) == 0 && !q.AllowOther {
			return nil, fmt.Errorf("question %d (%q) has no options; start option lines with \"-\"", bi+1, q.Prompt)
		}
		q.HelpMarkdown = strings.Join(help, "\n")
		out = append(out, q)
	}
	return out, nil
}

var mentionRe = regexp.MustCompile(`@?([a-z0-9._-]+)`)

func parseRespondents(s string) []string {
	var out []string
	for _, m := range mentionRe.FindAllStringSubmatch(strings.ToLower(s), -1) {
		if !contains(out, m[1]) {
			out = append(out, m[1])
		}
	}
	return out
}
