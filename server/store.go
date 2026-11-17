package main

import (
	"encoding/json"
	"fmt"
)

// KV layout (SPEC §5.4):
//   pack:{id}               -> Pack JSON
//   channel:{id}:packs      -> []packID
//   user:{id}:pending       -> []packID
// Callers that read-modify-write must hold p.kvLock.

func packKey(id string) string         { return "pack:" + id }
func channelIndexKey(id string) string { return "channel:" + id + ":packs" }
func userPendingKey(id string) string  { return "user:" + id + ":pending" }

func (p *Plugin) getPack(id string) (*Pack, error) {
	b, appErr := p.API.KVGet(packKey(id))
	if appErr != nil {
		return nil, appErr
	}
	if b == nil {
		return nil, nil
	}
	var pk Pack
	if err := json.Unmarshal(b, &pk); err != nil {
		return nil, fmt.Errorf("decode pack %s: %w", id, err)
	}
	if pk.Answers == nil {
		pk.Answers = map[string]map[string]Answer{}
	}
	return &pk, nil
}

func (p *Plugin) savePack(pk *Pack) error {
	b, err := json.Marshal(pk)
	if err != nil {
		return err
	}
	if appErr := p.API.KVSet(packKey(pk.ID), b); appErr != nil {
		return appErr
	}
	return nil
}

func (p *Plugin) getIndex(key string) ([]string, error) {
	b, appErr := p.API.KVGet(key)
	if appErr != nil {
		return nil, appErr
	}
	ids := []string{}
	if b != nil {
		if err := json.Unmarshal(b, &ids); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

func (p *Plugin) setIndex(key string, ids []string) error {
	b, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	if appErr := p.API.KVSet(key, b); appErr != nil {
		return appErr
	}
	return nil
}

func (p *Plugin) indexAdd(key, id string) error {
	ids, err := p.getIndex(key)
	if err != nil {
		return err
	}
	if contains(ids, id) {
		return nil
	}
	return p.setIndex(key, append(ids, id))
}

func (p *Plugin) indexRemove(key, id string) error {
	ids, err := p.getIndex(key)
	if err != nil {
		return err
	}
	out := ids[:0]
	for _, v := range ids {
		if v != id {
			out = append(out, v)
		}
	}
	return p.setIndex(key, out)
}
