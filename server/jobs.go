package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const sweepEvery = 10 * time.Minute

func (p *Plugin) sweepLoop(stop chan struct{}) {
	first := time.NewTimer(time.Minute)
	t := time.NewTicker(sweepEvery)
	defer first.Stop()
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-first.C:
			p.sweep(model.GetMillis())
		case <-t.C:
			p.sweep(model.GetMillis())
		}
	}
}

func (p *Plugin) allPackIDs() []string {
	var ids []string
	for page := 0; ; page++ {
		keys, appErr := p.API.KVList(page, 200)
		if appErr != nil || len(keys) == 0 {
			return ids
		}
		for _, k := range keys {
			if strings.HasPrefix(k, "pack:") {
				ids = append(ids, strings.TrimPrefix(k, "pack:"))
			}
		}
	}
}

func (p *Plugin) permalink(pk *Pack) string {
	site := ""
	if c := p.API.GetConfig(); c != nil && c.ServiceSettings.SiteURL != nil {
		site = strings.TrimRight(*c.ServiceSettings.SiteURL, "/")
	}
	team, appErr := p.API.GetTeam(pk.TeamID)
	if appErr != nil {
		return ""
	}
	return fmt.Sprintf("%s/%s/pl/%s", site, team.Name, pk.PostID)
}

// sweep expires packs past closes_at and sends one 24h reminder DM per pending respondent.
func (p *Plugin) sweep(now int64) {
	for _, id := range p.allPackIDs() {
		p.kvLock.Lock()
		pk, err := p.getPack(id)
		if err != nil || pk == nil {
			p.kvLock.Unlock()
			continue
		}
		expire, nudge := dueActions(pk, now)
		changed := false
		if expire {
			pk.Status = StatusExpired
			pk.LockedAt = &now
			changed = true
			p.clearPending(pk)
			p.updateCard(pk)
			p.postClose(pk, "⌛ Pack expired:")
		}
		for _, u := range nudge {
			if p.botID == "" {
				break
			}
			ch, appErr := p.API.GetDirectChannel(p.botID, u)
			if appErr == nil {
				_, _ = p.API.CreatePost(&model.Post{UserId: p.botID, ChannelId: ch.Id,
					Message: fmt.Sprintf("Reminder: **%s** is still waiting on your answers. %s", pk.Title, p.permalink(pk))})
			}
			pk.NudgedUserIDs = append(pk.NudgedUserIDs, u)
			changed = true
		}
		if changed {
			_ = p.savePack(pk)
		}
		p.kvLock.Unlock()
	}
}
