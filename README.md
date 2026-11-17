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
