# Throwaway dogfood rig

`rig.sh` starts `mattermost/mattermost-preview:12.0.2` bound to 127.0.0.1:8065 and creates generated test users
(admin/drew/scott, passwords in a local 0600 `creds` file, never committed), a `sigil` bot with a token, team `ssq`
and channel `office-hours`. It then installs the newest plugin tarball from the rig directory.
`a1.py` (Playwright) has the bot create a pack, checks the API, and screenshots the card as drew and scott.
This is never the production server.
