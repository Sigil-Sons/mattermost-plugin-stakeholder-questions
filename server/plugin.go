package main

import (
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// Plugin is the server half of Stakeholder Questions.
type Plugin struct {
	plugin.MattermostPlugin

	cfgLock sync.RWMutex
	cfg     *configuration

	// kvLock serialises read-modify-write of packs and indexes (single-node MM).
	kvLock sync.Mutex

	botID string
	stop  chan struct{}
}

// OnActivate registers /ssq, ensures the plugin bot (thread replies, reminders) and starts the sweeper.
func (p *Plugin) OnActivate() error {
	botID, err := p.API.EnsureBotUser(&model.Bot{Username: "stakeholder-questions", DisplayName: "Stakeholder Questions",
		Description: "Posts pack results and reminders."})
	if err != nil {
		return err
	}
	p.botID = botID
	if err := p.API.RegisterCommand(&model.Command{Trigger: "ssq", AutoComplete: true,
		AutoCompleteDesc: "Stakeholder question packs", AutoCompleteHint: "pending"}); err != nil {
		return err
	}
	p.stop = make(chan struct{})
	go p.sweepLoop(p.stop)
	return nil
}

func (p *Plugin) OnDeactivate() error {
	if p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
	return nil
}

type configuration struct {
	MaxQuestionsPerPack            int
	DefaultCloseDays               int
	AllowObserverResultsOnComplete bool
	AllowedTeamIds                 string
}

func (c *configuration) maxQuestions() int {
	if c == nil || c.MaxQuestionsPerPack <= 0 {
		return 8
	}
	return c.MaxQuestionsPerPack
}

func (c *configuration) teamAllowed(teamID string) bool {
	if c == nil || strings.TrimSpace(c.AllowedTeamIds) == "" {
		return true
	}
	for _, t := range strings.Split(c.AllowedTeamIds, ",") {
		if strings.TrimSpace(t) == teamID {
			return true
		}
	}
	return false
}

func (p *Plugin) config() *configuration {
	p.cfgLock.RLock()
	defer p.cfgLock.RUnlock()
	if p.cfg == nil {
		return &configuration{MaxQuestionsPerPack: 8, DefaultCloseDays: 7, AllowObserverResultsOnComplete: true}
	}
	return p.cfg
}

// OnConfigurationChange loads only this plugin's own settings block (never touches other plugins' config).
func (p *Plugin) OnConfigurationChange() error {
	c := &configuration{MaxQuestionsPerPack: 8, DefaultCloseDays: 7, AllowObserverResultsOnComplete: true}
	if err := p.API.LoadPluginConfiguration(c); err != nil {
		return err
	}
	p.cfgLock.Lock()
	p.cfg = c
	p.cfgLock.Unlock()
	return nil
}
