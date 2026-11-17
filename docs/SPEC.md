# Mattermost · Stakeholder Questions (multiplayer design/arch packs) — Spec

**Status:** draft for Quorra · **Product:** Drew · **PM:** KITT · **Build:** Quorra  
**Date:** 2026-11-17  
**Working title:** Sigil Stakeholder Questions (SSQ)  
**Plugin id (proposed):** `com.mysticspark.stakeholder-questions`  
**Surface:** Mattermost only (Sigil & Sons team first) · not Desktop · not Kaneo unfurl  

**Inspiration:** Claude Desktop `AskUserQuestion` multi-step cards (single/multi select, Recommended, Other, context images) + Mattermost poll/quorum + @mentions.

---

## 0. Grill log (Drew 2026-11-17 — locked via question tool)

| # | Decision | MVP lock |
|---|----------|----------|
| G1 | Delivery UX | **Compact custom post card + wizard (modal/RHS)** |
| G2 | Create path | **Bot API primary** + human slash optional |
| G3 | Respondents / quorum | **Configurable.** Default for 2 stakeholders = all @mentioned. Pack may set `min_respondents` / N-of-M; **director agent chooses using question/pack importance** (high → require all; lower → allow N-of-M) |
| G4 | See others’ answers | **Hidden by default**; optional control **“Show others’ answers”** when the user wants (not forced live) |
| G5 | Consensus | **Independent answers** — histogram + names; no force-agree in v1 |
| G6 | Images | **All three:** context/help · option mockups · answer images (MM files / Keep only) |
| G7 | Storage | **Plugin KV + markdown summary export**; Kaneo/Keep export = v1.1 |
| G8 | Revise | **Editable until pack locks** (quorum met or author lock) |
| G9 | Scott OPSEC | No house topology / Money Bin / care in Scott-visible packs |
| G10 | Notifications | @mention on create; light nudge if still pending (optional) |

**Still soft (Quorra may use defaults):**
- O1: Max questions per pack = **8**
- O2: Auto-close = **7 days** or manual
- O3: Anonymous = **no** (always named in Sigil)

---

## 1. Goal

Let **@sigil** (or another director bot) post a **scannable, interactive question pack** in a Sigil MM channel that:

1. Tags required stakeholders (e.g. **@drew** + **@scott**).  
2. Presents **N design/architecture/production questions** with enough context (markdown + images).  
3. Collects answers from **multiple people** without drowning the channel in thread noise.  
4. Shows **progress** (who’s done / who’s pending) and, once you’ve answered a question, **what others chose**.  
5. Produces a **durable decision record** the bots can read back.

Mental model: *AskUserQuestion pack* × *quorum poll* × *decision log*.

---

## 2. Non-goals (v1)

- Replacing Jira/Kaneo as system of record for all work  
- Real-time collaborative whiteboard  
- Anonymous voting  
- Public internet forms (LAN MM only is fine)  
- Full SurveyMonkey logic (branching trees beyond linear steps — **linear wizard only** in v1)  
- Desktop Hermes `::` directives (optional later)  
- Forcing Scott into house-only channels  

---

## 3. Users & flows

### 3.1 Actors

| Actor | Role |
|-------|------|
| **Author** | Bot (`@sigil`) or human who creates the pack |
| **Respondent** | @mentioned users who must answer |
| **Observer** | Channel members who can see pack status but aren’t required |
| **Agent consumer** | Hermes profiles reading pack JSON via plugin API |

### 3.2 Create flow (bot)

1. Agents building product accumulate questions (structured JSON).  
2. Director bot validates schema → `POST /plugins/.../api/v1/packs`.  
3. Plugin creates a **custom post** in target channel with props + mentions.  
4. MM notifies mentioned users.

### 3.3 Answer flow (human)

1. User sees compact **channel card**: title, progress chips, CTA **Answer (2/5 done)**.  
2. Click opens **modal or RHS panel** (wizard, 1 question at a time or list — see §4).  
3. Each question: prompt, help markdown, optional images, choices (+ Other + optional image upload).  
4. Others’ answers **hidden by default**; user may tap **Show others’ answers** on a question when unsure / curious.  
5. When user finishes all questions → card updates their chip to ✅.  
6. When quorum met → pack status **Complete**; optional summary post/thread.

### 3.4 Agent read-back

`GET /api/v1/packs/{id}` → full answers by user/question for director bot to continue build.

---

## 4. UX specification

### 4.1 Channel card (always visible)

Compact custom post body (custom post type `custom_ssq_pack`):

```
📋 Stakeholder pack · StoryBard deployment topology
Need: @drew @scott · 3 questions · Quorum 2/2
Progress: drew ✅ · scott ⏳
[Answer questions]  [View results]
Status: Open · created by @sigil · 2026-11-17
```

- Color bar by status: Open / In progress / Complete / Expired  
- **Do not** dump all questions inline in the channel (defeats the goal)  
- Optional one-line **pack summary** (author-provided)

### 4.2 Answer UI (wizard)

Match AskUserQuestion density:

- **Q i of N** header  
- Question title  
- Explanation (markdown; images via MM file ids or HTTPS URLs on Keep/house hosts — **no catbox**)  
- Radio **or** checkbox options  
- First option may be marked **(Recommended)** visually  
- **Other** free-text row when enabled  
- Option may include **thumbnail** (mockup)  
- Footer: Skip (if allowed) · Back · Next / Submit  
- **Show others’ answers** control (default collapsed/hidden); expands avatars + choice labels when user opts in  


**v1 recommendation:** RHS panel (persistent while navigating channel) *or* interactive dialog/modal — Quorra picks based on MM 12 plugin APIs that work with React 19 (see Jitsi sigil.1 lesson: no `ReactDOM.render`). Prefer **`registerRootComponent` / official interactive dialogs** over unsupported portals.

### 4.3 Results view

- **View results** enabled for respondents always after *their* completion; for observers when pack Complete (configurable).  
- Per-question histogram + named breakdown.  
- Export button: copy markdown summary.

---

## 5. Data model

### 5.1 Pack

```json
{
  "id": "pack_…",
  "team_id": "…",
  "channel_id": "…",
  "post_id": "…",
  "title": "StoryBard deployment topology",
  "summary": "Optional one-liner",
  "author_user_id": "…",
  "author_bot": true,
  "respondent_user_ids": ["drew_id", "scott_id"],
  "min_respondents": 2,
  "importance": "high",
  "visibility": "others_opt_in",

  "allow_skip": false,
  "allow_revise_until_lock": true,
  "status": "open",
  "created_at": 0,
  "closes_at": null,
  "locked_at": null,
  "questions": [ /* Question */ ],
  "answers": { /* user_id → { question_id → Answer } */ },
  "completed_user_ids": []
}
```

### 5.2 Question

```json
{
  "id": "q1",
  "prompt": "Which data warehouse…?",
  "help_markdown": "Context…\n\n![arch](…)",
  "help_file_ids": [],
  "selection": "single",
  "allow_other": true,
  "options": [
    {
      "id": "snowflake",
      "label": "Snowflake",
      "description": "Mature, separates storage and compute…",
      "recommended": true,
      "image_file_id": null,
      "image_url": null
    }
  ]
}
```

`selection`: `single` | `multi`  
v1.1: `text_only`, `require_unanimous`

### 5.3 Answer

```json
{
  "option_ids": ["snowflake"],
  "other_text": null,
  "image_file_ids": [],
  "updated_at": 0
}
```

### 5.4 Storage

- Mattermost **plugin KV** (JSON blobs) keyed `pack:{id}`  
- Index: `channel:{id}:packs` → ids  
- Index: `user:{id}:pending` → pack ids  
- Files: use MM file upload API; store file ids on question/answer  

---

## 6. API (plugin HTTP)

Auth: Mattermost session user **or** bot token; pack create also allowed via plugin bot.

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/packs` | Create pack + channel post |
| GET | `/api/v1/packs/{id}` | Full pack (respect visibility rules) |
| POST | `/api/v1/packs/{id}/answers` | Submit/revise answers `{question_id, option_ids, other_text, image_file_ids}` |
| POST | `/api/v1/packs/{id}/lock` | Author/admin lock |
| GET | `/api/v1/packs/{id}/summary.md` | Markdown decision log |
| GET | `/api/v1/me/pending` | Packs waiting on current user |

**Create body (bot):** title, channel_id, respondent usernames/ids, questions[], optional closes_at, min_respondents.

On create: validate ≤ max questions; resolve usernames; create post; set props `ssq_pack_id`.

---

## 7. Mattermost integration notes (Quorra)

### 7.1 Patterns to use

- **Custom post type** + `registerPostTypeComponent` for channel card  
- **Interactive buttons** on the post → open answer UI  
- **Webapp** React 19-safe mounting (see `jitsi-2.1.0-sigil.1` / `registerRootComponent`)  
- Externalize React like Kaneo plugin  

### 7.2 Patterns to avoid

- Stuffing 8 full AskUserQuestion UIs into attachment fields (unreadably tall)  
- `ReactDOM.render`  
- Public paste hosts for mockups — **Keep / MM files only**  
- Replacing entire `PluginSettings.Plugins` on install (Jitsi wipe incident)  

### 7.3 Existing cousins

- Kaneo plugin `com.mysticspark.kaneo` — unfurl/hover only; **do not overload it**  
- Matterpoll — simple polls; insufficient context/images/quorum UX  
- This is a **new plugin**

### 7.4 Slash (nice-to-have v1)

`/ssq pending` — list packs waiting on you  
`/ssq new` — human author stub (optional; bot API is enough for MVP)

---

## 8. Notifications

| Event | Action |
|-------|--------|
| Pack created | Post @mentions respondents |
| You still pending after 24h | Optional ephemeral/DM once |
| Pack complete | Update card; short thread reply “Pack complete — summary” + link |
| Revise | Update card only; no spam |

---

## 9. Permissions & OPSEC

- Create: channel members with post permission + bots  
- Answer: only `respondent_user_ids` (others View results per rules)  
- Lock: author or team admin  
- **Sigil channels:** packs may include product mockups; still **no** house IPs/CTs/Money Bin/Baymax care  
- Audit: store answer timestamps + user ids  

---

## 10. Agent contract (for @sigil / Quorra ISO)

Director bot collects from builder agents a JSON array of questions (same schema as §5.2), then:

```http
POST /plugins/com.mysticspark.stakeholder-questions/api/v1/packs
```

Poll `GET …/packs/{id}` until `status=complete`, then continue implementation from `answers`.

Hermes skill (follow-up): `sigil-stakeholder-questions` with example payload.

---

## 11. Acceptance (MVP)

A1. Bot creates pack in `#office-hours` (or test channel) mentioning Drew+Scott; card shows 2 pending.  
A2. Drew opens Answer UI; completes all questions; sees Scott empty until Scott answers.  
A3. Scott answers; both see each other’s choices per question.  
A4. Card shows 2/2 complete; summary.md lists decisions.  
A5. Image context on a question renders; one option is a mockup image.  
A6. Multi-select question stores multiple option_ids.  
A7. Revise before lock updates answer.  
A8. Plugin install does not clobber Jitsi/Kaneo settings (assert scripts still green).  
A9. React 19: no blank card / no `render is not a function`.  

---

## 12. Implementation sketch

```
mattermost-plugin-stakeholder-questions/
  plugin.json
  server/   # packs CRUD, KV, post create/update, authz
  webapp/   # PostType card, Answer wizard, Results
  build.sh
```

**Suggested slices for Quorra**

1. Schema + KV + create pack API + plain custom post stub  
2. Channel card UI + progress  
3. Answer wizard (single/multi/other)  
4. Visibility rules + results  
5. Images (MM files)  
6. Bot example + A1–A9 dogfood with Drew/Scott  
7. Polish: DM nudge, summary.md, lock  

---

## 13. Config (System Console)

| Key | Default | Notes |
|-----|---------|-------|
| MaxQuestionsPerPack | 8 | |
| DefaultCloseDays | 7 | 0 = no auto-close |
| AllowObserverResultsOnComplete | true | |
| AllowedTeamIds | Sigil team id | empty = all (dev only) |

---

## 14. Risks

| Risk | Mitigation |
|------|------------|
| MM interactive UI limited | Prefer custom post + RHS/root; spike day-1 |
| Card noise | One card per pack; no per-answer channel spam |
| Scott overloaded | min questions; strong help text; Recommended defaults |
| Bot spam packs | Rate limit create per channel; require title |
| React 19 | Copy Jitsi sigil mount pattern |

---

## 15. Ticket / handoff

- Kaneo: create under **SS** board (Sigil & Sons)  
- Spec path: `/mnt/labs/sigil/docs/MM-STAKEHOLDER-QUESTIONS-SPEC.md`  
- Implementer: **Quorra** · PM accept: **KITT** · E2E: Drew+Scott  

---

## 16. Drew overrides

Reply with overrides to G1–G10 or O1–O4; KITT will patch this doc and hand Quorra.
