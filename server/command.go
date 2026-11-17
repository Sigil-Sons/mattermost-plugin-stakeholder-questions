package main

import (
	"fmt"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin"
)

// ExecuteCommand handles /ssq [pending|help] (SPEC §7.4).
func (p *Plugin) ExecuteCommand(_ *plugin.Context, args *model.CommandArgs) (*model.CommandResponse, *model.AppError) {
	fields := strings.Fields(args.Command)
	sub := "pending"
	if len(fields) > 1 {
		sub = strings.ToLower(fields[1])
	}
	reply := func(t string) (*model.CommandResponse, *model.AppError) {
		return &model.CommandResponse{ResponseType: model.CommandResponseTypeEphemeral, Text: t}, nil
	}
	if sub == "new" {
		if appErr := p.openCreateDialog(args.TriggerId, args.ChannelId); appErr != nil {
			return reply("Could not open the form: " + appErr.Error())
		}
		return &model.CommandResponse{}, nil
	}
	if sub != "pending" {
		return reply("`/ssq new` opens a form to post a question pack here. `/ssq pending` lists packs waiting on you. Bots can post packs through the plugin API (docs/AGENT.md).")
	}
	ids, err := p.getIndex(userPendingKey(args.UserId))
	if err != nil {
		return reply("Could not load your pending packs.")
	}
	var lines []string
	for _, id := range ids {
		pk, err := p.getPack(id)
		if err != nil || pk == nil || pk.Status != StatusOpen || contains(pk.CompletedUserIDs, args.UserId) {
			continue
		}
		lines = append(lines, fmt.Sprintf("- **%s** · %d/%d answered · %s", pk.Title,
			len(pk.Answers[args.UserId]), len(pk.Questions), p.permalink(pk)))
	}
	if len(lines) == 0 {
		return reply("Nothing waiting on you. 🎉")
	}
	return reply("**Question packs waiting on you**\n" + strings.Join(lines, "\n"))
}
