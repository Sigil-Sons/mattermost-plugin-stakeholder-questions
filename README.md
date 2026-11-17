# Mattermost Stakeholder Questions

Multiplayer **question packs** for Mattermost: AskUserQuestion-style design/arch/production decisions with **quorum**, **@respondents**, optional **show others’ answers**, and a **bot API** so director agents (e.g. Sigil) can collect stakeholder input without drowning the channel.

Built for [Sigil & Sons](https://github.com/Sigil-Sons) · reusable on any Mattermost 12+ team.

## Status

**Spec complete · implementation in progress (Quorra / SS-53).**

Product/build spec: [`docs/SPEC.md`](docs/SPEC.md)  
House copy: `/mnt/labs/sigil/docs/MM-STAKEHOLDER-QUESTIONS-SPEC.md`

## Features (MVP)

- Compact **channel card** + **answer wizard** (modal/RHS)
- Single / multi select, **Recommended**, **Other**, markdown + **images** (context, option mockups, answer images)
- Configurable quorum (importance-driven N-of-M or all mentioned)
- Others’ answers **hidden by default** · opt-in “Show others”
- Independent answers (no force-agree)
- Plugin KV storage + markdown summary export
- Bot HTTP API for pack create / poll results
- React 19–safe webapp (Mattermost 12)

## Plugin id

`com.mysticspark.stakeholder-questions`

## License

MIT

## Status

**WIP: slice 1** (SS-53). Server: create pack, read pack (visibility-filtered), my pending. Webapp: compact channel card (read-only stub).
Answer wizard, answers API, results, images, lock and summary.md arrive in later slices (see `docs/SPEC.md` §12).

## Build

```bash
./build.sh   # Go (linux amd64/arm64) + esbuild webapp in throwaway containers -> dist/<id>-<version>.tar.gz
```

Webapp uses Mattermost's own React (`window.React`, React 19 on MM 12) through an esbuild alias and only supported
registry APIs (`registerPostTypeComponent`). No `ReactDOM.render`, no second React copy.

## API (slice 1)

All under `/plugins/com.mysticspark.stakeholder-questions/api/v1`, authenticated with a session or bot token.

| Method | Path | Notes |
|---|---|---|
| POST | `/packs` | `{title, channel_id, respondents:[usernames or ids], questions:[…], importance?, min_respondents?, summary?, closes_at?}` → 201 pack; posts a `custom_ssq_pack` card that @mentions respondents |
| GET | `/packs/{id}` | Author/admin: all answers. Respondent: own answers; others' only with `?show_others=1` (opt-in, never auto-revealed). Observer: results only when complete (configurable). |
| GET | `/me/pending` | Open packs waiting on the caller |
