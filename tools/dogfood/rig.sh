#!/usr/bin/env bash
# CT305: throwaway Mattermost 12.0.2 (localhost only) for SS-53 dogfood. Not prod. Idempotent-ish.
set -euo pipefail
D=/root/ssq-dogfood; mkdir -p $D; cd $D; umask 077
TGZ=$(ls -t $D/com.mysticspark.stakeholder-questions-*.tar.gz | head -1)
if ! docker ps --format '{{.Names}}' | grep -qx ssq-mm; then
  docker rm -f ssq-mm >/dev/null 2>&1 || true
  docker run -d --name ssq-mm -p 127.0.0.1:8065:8065 \
    -e MM_SERVICESETTINGS_ENABLELOCALMODE=true -e MM_PLUGINSETTINGS_ENABLEUPLOADS=true \
    -e MM_SERVICESETTINGS_ENABLEBOTACCOUNTCREATION=true -e MM_SERVICESETTINGS_ENABLEUSERACCESSTOKENS=true \
    -e MM_SERVICESETTINGS_SITEURL=http://127.0.0.1:8065 \
    mattermost/mattermost-preview:12.0.2 >/dev/null
fi
for i in $(seq 1 90); do curl -sf http://127.0.0.1:8065/api/v4/system/ping >/dev/null && break; sleep 3; done
curl -s http://127.0.0.1:8065/api/v4/system/ping; echo
MMCTL=$(docker exec ssq-mm sh -c 'command -v mmctl || find / -name mmctl -type f 2>/dev/null | head -1')
for i in $(seq 1 30); do docker exec ssq-mm $MMCTL --local system version >/dev/null 2>&1 && break; sleep 2; done
m() { docker exec ssq-mm $MMCTL --local "$@"; }
[ -f creds ] || { for u in admin drew scott; do echo "$u=$(head -c 18 /dev/urandom | base64 | tr -d '/+=')Aa1!"; done > creds; }
pw() { grep "^$1=" creds | cut -d= -f2-; }
m user create --email admin@ssq.test --username admin --password "$(pw admin)" --system-admin >/dev/null 2>&1 || true
m user create --email drew@ssq.test --username drew --password "$(pw drew)" >/dev/null 2>&1 || true
m user create --email scott@ssq.test --username scott --password "$(pw scott)" >/dev/null 2>&1 || true
m team create --name ssq --display-name "SSQ Dogfood" >/dev/null 2>&1 || true
m team users add ssq admin drew scott >/dev/null 2>&1 || true
m channel create --team ssq --name office-hours --display-name "Office Hours" >/dev/null 2>&1 || true
# bots can't be created in local mode: use REST as the throwaway admin
if [ ! -s bot.tok ]; then ADMINPW="$(pw admin)" python3 - <<'PY' > bot.tok.new
import json, os, urllib.request
MM = "http://127.0.0.1:8065/api/v4"
def call(method, path, tok=None, body=None):
    req = urllib.request.Request(MM + path, method=method, data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Content-Type": "application/json", **({"Authorization": "Bearer " + tok} if tok else {})})
    with urllib.request.urlopen(req) as r:
        return r.headers, json.loads(r.read() or b"{}")
h, _ = call("POST", "/users/login", body={"login_id": "admin", "password": os.environ["ADMINPW"]})
tok = h["Token"]
try:
    bot = call("POST", "/bots", tok, {"username": "sigil", "display_name": "Sigil"})[1]
    bid = bot["user_id"]
except urllib.error.HTTPError:
    bid = call("GET", "/users/username/sigil", tok)[1]["id"]
print(call("POST", f"/users/{bid}/tokens", tok, {"description": "ssq-dogfood"})[1]["token"])
PY
mv bot.tok.new bot.tok; fi
m team users add ssq sigil >/dev/null 2>&1 || true
m channel users add ssq:office-hours drew scott sigil >/dev/null 2>&1 || true
# A8: seed another plugin's settings, install, and prove they survive (Jitsi wipe incident)
printf '{"PluginSettings":{"Plugins":{"jitsi":{"jitsiurl":"https://meet.example.test","jitsinamingscheme":"mattermost"}}}}' > a8.json
docker cp a8.json ssq-mm:/tmp/a8.json; m config patch /tmp/a8.json >/dev/null
BEFORE=$(m config get PluginSettings.Plugins.jitsi 2>/dev/null | tr -d ' \n')
docker cp "$TGZ" ssq-mm:/tmp/ssq.tar.gz
m plugin add /tmp/ssq.tar.gz --force 2>&1 | tail -1
m plugin enable com.mysticspark.stakeholder-questions 2>&1 | tail -1
# verify the installed webapp bundle is the one we shipped (a silent stale install bit us once)
WANT=$(tar -xzOf "$TGZ" com.mysticspark.stakeholder-questions/webapp/dist/main.js | md5sum | cut -d' ' -f1)
HAVE=$(docker exec ssq-mm md5sum /mm/mattermost/plugins/com.mysticspark.stakeholder-questions/webapp/dist/main.js | cut -d' ' -f1)
echo "installed $(basename "$TGZ") bundle_match=$([ "$WANT" = "$HAVE" ] && echo yes || echo NO)"
[ "$WANT" = "$HAVE" ]
AFTER=$(m config get PluginSettings.Plugins.jitsi 2>/dev/null | tr -d ' \n')
echo "A8 other plugin config preserved=$([ -n "$BEFORE" ] && [ "$BEFORE" = "$AFTER" ] && echo yes || echo NO) ($AFTER)"
