package main

import (
	_ "embed"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

//go:embed assets/answer.html
var answerHTML []byte

//go:embed assets/answer.js
var answerJS []byte

//go:embed assets/answer.css
var answerCSS []byte

const pluginPath = "/plugins/com.mysticspark.stakeholder-questions"

func (p *Plugin) siteURL() string {
	if c := p.API.GetConfig(); c != nil && c.ServiceSettings.SiteURL != nil {
		return strings.TrimRight(*c.ServiceSettings.SiteURL, "/")
	}
	return ""
}

func (p *Plugin) answerLink(packID string) string {
	return p.siteURL() + pluginPath + "/answer/" + packID
}

// serveMobile handles the standalone answer page + its assets. Returns true if it handled the request.
// No pack data is embedded in the page; the page fetches it with the user's own session.
func (p *Plugin) serveMobile(w http.ResponseWriter, r *http.Request, userID string) bool {
	path := r.URL.Path
	if r.Method != http.MethodGet {
		return false
	}
	switch {
	case path == "/answer.js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(answerJS)
		return true
	case path == "/answer.css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(answerCSS)
		return true
	case strings.HasPrefix(path, "/answer/"):
		if userID == "" {
			// send them through the normal Mattermost login and straight back here
			back := pluginPath + path
			http.Redirect(w, r, p.siteRoot()+"/login?redirect_to="+url.QueryEscape(back), http.StatusFound)
			return true
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		_, _ = w.Write(answerHTML)
		return true
	}
	return false
}

// siteRoot is the SiteURL path prefix (subpath installs), e.g. "" or "/mm".
func (p *Plugin) siteRoot() string {
	u, err := url.Parse(p.siteURL())
	if err != nil {
		return ""
	}
	return strings.TrimRight(u.Path, "/")
}

// fallbackMessage is what the Mattermost mobile app (no plugin webapp), push, email and search see.
// The web card hides it. It lists the questions and links to the standalone answer page.
func fallbackMessage(pk *Pack, names []string, link string) string {
	at := make([]string, len(names))
	for i, n := range names {
		at[i] = "@" + n
	}
	var b strings.Builder
	fmt.Fprintf(&b, "📋 **Stakeholder pack · %s**: %s, %d question(s) waiting on you.\n", pk.Title, strings.Join(at, " "), len(pk.Questions))
	if pk.Summary != "" {
		fmt.Fprintf(&b, "_%s_\n", pk.Summary)
	}
	for i, q := range pk.Questions {
		kind := "pick one"
		if q.Selection == SelectionMulti {
			kind = "pick any"
		}
		opts := make([]string, 0, len(q.Options)+1)
		for _, o := range q.Options {
			l := o.Label
			if o.Recommended {
				l += " (recommended)"
			}
			opts = append(opts, l)
		}
		if q.AllowOther {
			opts = append(opts, "Other")
		}
		fmt.Fprintf(&b, "%d. %s _(%s)_: %s\n", i+1, q.Prompt, kind, strings.Join(opts, " · "))
	}
	if link != "" {
		fmt.Fprintf(&b, "\n📱 [Answer in browser](%s)", link)
	}
	return b.String()
}
