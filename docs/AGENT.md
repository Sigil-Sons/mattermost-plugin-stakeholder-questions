# Agent contract: posting a stakeholder pack

For director bots (for example `@sigil`) that gather questions from builder agents. SPEC §10.

## Create

```http
POST {MM}/plugins/com.mysticspark.stakeholder-questions/api/v1/packs
Authorization: Bearer <bot token>
Content-Type: application/json
```

```json
{
  "title": "StoryBard deployment topology",
  "summary": "Three calls we need before T22",
  "channel_id": "<office-hours channel id>",
  "respondents": ["drew", "scott"],
  "importance": "high",
  "questions": [
    {
      "prompt": "Where should the model bridge run?",
      "help_markdown": "Context: the bridge talks to **Blade** for inference.",
      "help_file_ids": ["<file id uploaded by this bot>"],
      "allow_other": true,
      "options": [
        {"label": "Hermes LXC", "recommended": true, "description": "Next to the gateway"},
        {"label": "Blade"}
      ]
    },
    {
      "prompt": "Which audio features ship in v1?",
      "selection": "multi",
      "options": [{"label": "TTS narration"}, {"label": "Ambient music"}, {"label": "SFX"}]
    }
  ]
}
```

Rules:

- 1–8 questions (System Console `MaxQuestionsPerPack`). Each needs 2+ options, or 1 + `allow_other`.
- Question ids default to `q1…`; option ids default to a slug of the label (`"Hermes LXC"` becomes `hermes-lxc`).
- Respondents are usernames or user ids, and they must already be in the channel.
- Quorum: `min_respondents` if given. Otherwise `importance: high`, or two or fewer respondents, means everyone; otherwise a majority.
- Images: upload with `POST /api/v4/files` (as the bot, `channel_id` = the target channel) and pass the file ids
  (`help_file_ids`, `options[].image_file_id`). `image_url` must be https on a house host; public paste hosts are rejected.
- The plugin posts the channel card and @mentions respondents. The response is the pack (`id`, `post_id`).

## Read back

When the pack completes, is locked or expires, the plugin replies in the card's thread with the decision log and
**@mentions the bot that created it** (bots only; human authors are not pinged). Treat that mention as the wake-up,
then pull the answers:


`GET …/packs/{id}` as the author returns every answer:
`answers[user_id][question_id] = {option_ids, other_text, image_file_ids, skipped}`.
Poll until `status` is `complete` (quorum met), `locked` (author locked) or `expired`.
`GET …/packs/{id}/summary.md` is the human-readable decision log.

## Other calls

| Call | Who |
|---|---|
| `POST …/packs/{id}/lock` | author or admin; freezes answers and posts the summary in the thread |
| `POST …/packs/{id}/answers` | respondents only (the wizard uses this) |
| `GET …/me/pending` | packs waiting on the caller |

Don't put house topology, Money Bin or care details in Scott-visible packs (SPEC §9).
