package main

import (
	"strings"
	"sync"

	"github.com/mattermost/mattermost/server/public/plugin"
)

// Plugin is the server half of Stakeholder Questions.
type Plugin struct {
	plugin.MattermostPlugin

	cfgLock sync.RWMutex
	cfg     *configuration

	// kvLock serialises read-modify-write of packs and indexes (single-node MM).
	kvLock sync.Mutex
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
