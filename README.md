# Mattermost Stakeholder Questions

Multiplayer **question packs** for Mattermost: AskUserQuestion-style design/arch/production decisions with **quorum**, **@respondents**, optional **show others’ answers**, and a **bot API** so director agents (e.g. Sigil) can collect stakeholder input without drowning the channel.

Built for [Sigil & Sons](https://github.com/Sigil-Sons) · reusable on any Mattermost 12+ team.

## Status

**v0.11.0** (SS-53, SPEC §11 A1–A9 proven on a throwaway MM 12.0.2). Awaiting KITT review,
then Drew + Scott dogfood on the house server.

- Bot API creates packs and posts a compact channel card that @mentions respondents and updates live as people finish
- Answer wizard: one question at a time, single/multi/Other, Recommended tag, markdown help, context/option/answer images,
  skip (if allowed), revise until the pack closes
- Others' answers hidden by default; "Show others' answers" reveals only submitted answers (opt-in, never automatic)
- Quorum completes and locks the pack and posts the summary in the thread; authors/admins can lock early; auto-expiry; one 24h reminder DM
- Results view: per-question histogram with names, plus Copy/Open `summary.md`
- `/ssq pending`
- Mattermost mobile app: the post text lists the questions and links to **📱 Answer in browser**, a lightweight
  answer page the plugin serves itself (`/plugins/com.mysticspark.stakeholder-questions/answer/<pack id>`, uses the normal
  Mattermost login). The native app can't run plugin web code, so this is the phone path.
- React 19 safe: only `registerPostTypeComponent` / `registerRootComponent`, no `ReactDOM.render`, no bundled React

Agent payloads and read-back: [`docs/AGENT.md`](docs/AGENT.md).

## Build

```bash
./build.sh   # Go (linux amd64/arm64) + esbuild webapp in throwaway containers -> dist/<id>-<version>.tar.gz
```

## Test

- `go test ./...` (in `build.sh`): validation, quorum/lock, visibility, summary, expiry/reminders
- `tools/dogfood/`: throwaway Mattermost 12.0.2 rig plus a Playwright end-to-end run as two users (`e2e.py`, 20 checks)

## Install notes

Upload the tarball under System Console → Plugins. The plugin only reads its own settings block and never writes
`PluginSettings.Plugins` (checked in the rig: another plugin's settings survive install). Set `AllowedTeamIds` to the
Sigil team in production. On first activation it creates a `stakeholder-questions` bot for thread summaries and reminders.
