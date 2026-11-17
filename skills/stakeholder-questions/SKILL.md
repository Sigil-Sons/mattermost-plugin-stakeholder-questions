---
name: stakeholder-questions
description: Use when you need decisions from people in Mattermost. Post a question pack, wait, read answers.
---

# Stakeholder question packs (Mattermost)

Any bot or person in a channel can post a **question pack**: a compact card the respondents answer in a wizard.
Use this instead of dumping a long list of questions into chat.

Plugin: `com.mysticspark.stakeholder-questions` (repo: Sigil-Sons/mattermost-plugin-stakeholder-questions).

## Setup

- `MATTERMOST_URL` (e.g. `https://mm.example.com`) and `MATTERMOST_TOKEN` (this bot's own token) in the environment.
- The bot must be a member of the target channel, and so must every respondent.
- Script: `scripts/ssq.py` (Python 3 stdlib only).

## Flow

1. Collect 1–8 decisions you are actually blocked on. Each question needs 2+ real options, or 1 + "Other".
   Mark the option you'd pick as `"recommended": true`, and say why in `description`.
2. Write `pack.json` (schema below), then:
   `python3 scripts/ssq.py create pack.json --channel <team>/<channel>` and keep the printed `pack_id`.
3. Stop and end your turn. Don't busy-wait. When everyone has answered (quorum), the plugin replies in the card's
   thread and **@mentions you**. That mention is your wake-up.
4. `python3 scripts/ssq.py get <pack_id>` gives JSON answers (`answers[user_id][question_id]`), and
   `python3 scripts/ssq.py summary <pack_id>` gives the markdown decision log. Continue the work from the answers.
   Treat an `Other` free-text answer as the real answer.

For short scripted runs only: `python3 scripts/ssq.py wait <pack_id> --timeout 600`.

## pack.json

```json
{
  "title": "StoryBard deployment topology",
  "summary": "Three calls needed before T22",
  "respondents": ["drew", "scott"],
  "importance": "high",
  "questions": [
    {"prompt": "Where should the model bridge run?",
     "help_markdown": "It talks to **Blade** for inference.",
     "allow_other": true,
     "options": [{"label": "Hermes LXC", "recommended": true, "description": "next to the gateway"},
                 {"label": "Blade"}]},
    {"prompt": "Which audio features ship in v1?", "selection": "multi",
     "options": [{"label": "TTS narration"}, {"label": "Ambient music"}, {"label": "SFX"}]}
  ]
}
```

- `importance: high` means everyone must answer. Otherwise use `min_respondents` for N-of-M.
- Images: `python3 scripts/ssq.py upload <team>/<channel> mock.png` prints a file id. Put it in `help_file_ids`
  (context) or `options[].image_file_id` (mockup). No public paste hosts.

## Rules

- Answers from others are hidden until each person opts in. Don't paraphrase one person's answer to another.
- Don't put private house details (IPs, finances, health) in packs that other people can see.
- People can make packs too: `/ssq new` in a channel opens a form. `/ssq pending` lists what's waiting on you.
