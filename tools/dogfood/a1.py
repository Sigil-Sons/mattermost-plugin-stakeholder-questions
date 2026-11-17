"""SS-53 A1 dogfood on throwaway MM: bot creates pack mentioning drew+scott; screenshot card as drew and scott;
check API visibility. Runs in loot-runner image with --network host."""
import asyncio, json, urllib.request

MM = "http://127.0.0.1:8065"
P = MM + "/plugins/com.mysticspark.stakeholder-questions/api/v1"
creds = dict(l.strip().split("=", 1) for l in open("/out/creds") if "=" in l)
BOT = open("/out/bot.tok").read().strip()


def api(method, url, tok, body=None):
    req = urllib.request.Request(url, method=method, data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Authorization": "Bearer " + tok, "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req) as r:
            return r.status, json.loads(r.read() or b"{}")
    except urllib.error.HTTPError as e:
        return e.code, json.loads(e.read() or b"{}")


def login(u):
    req = urllib.request.Request(MM + "/api/v4/users/login", method="POST",
                                 data=json.dumps({"login_id": u, "password": creds[u]}).encode(),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req) as r:
        return r.headers["Token"], json.loads(r.read())["id"]


ch = api("GET", MM + "/api/v4/teams/name/ssq/channels/name/office-hours", BOT)[1]
code, pack = api("POST", P + "/packs", BOT, {
    "title": "StoryBard deployment topology", "summary": "Dogfood pack for SS-53 slice 1",
    "channel_id": ch["id"], "respondents": ["@drew", "scott"], "importance": "high",
    "questions": [
        {"prompt": "Where should the model bridge run?", "options": [
            {"label": "Hermes LXC", "recommended": True}, {"label": "Blade"}], "allow_other": True},
        {"prompt": "Which audio features ship in v1?", "selection": "multi", "options": [
            {"label": "TTS narration"}, {"label": "Ambient music"}, {"label": "SFX"}]},
        {"prompt": "Subscription model?", "options": [{"label": "Per table"}, {"label": "Per DM seat"}]},
    ]})
print("create:", code, "pack", pack.get("id"), "post", pack.get("post_id"), "min", pack.get("min_respondents"),
      "qids", [q["id"] for q in pack.get("questions", [])])
bad = api("POST", P + "/packs", BOT, {"title": "x", "channel_id": ch["id"], "respondents": ["drew"],
                                      "questions": [{"prompt": "p", "options": [{"label": "only"}]}]})
print("reject 1-option:", bad[0], bad[1].get("error"))
dtok, did = login("drew")
print("drew GET:", api("GET", P + "/packs/" + pack["id"], dtok)[0], "| drew pending:",
      [(x["title"], x["questions"]) for x in api("GET", P + "/me/pending", dtok)[1]])
post = api("GET", MM + "/api/v4/posts/" + pack["post_id"], BOT)[1]
print("post type:", post.get("type"), "| message:", post.get("message"))


async def shots():
    from playwright.async_api import async_playwright
    async with async_playwright() as p:
        b = await p.chromium.launch(args=["--no-sandbox"])
        for u in ("drew", "scott"):
            tok, uid = login(u)
            ctx = await b.new_context(viewport={"width": 1300, "height": 820})
            await ctx.add_cookies([{"name": "MMAUTHTOKEN", "value": tok, "url": MM},
                                   {"name": "MMUSERID", "value": uid, "url": MM}])
            await ctx.add_init_script("try{localStorage.setItem('__landingPageSeen__','true')}catch(e){}")
            pg = await ctx.new_page()
            errs = []
            pg.on("pageerror", lambda e: errs.append(str(e)[:160]))
            await pg.goto(MM + "/ssq/channels/office-hours")
            card = pg.locator('[data-testid="ssq-card"]').last
            try:
                await card.wait_for(timeout=30000)
                txt = (await card.inner_text()).replace("\n", " | ")
                txt += " | borderLeft=" + await card.evaluate("e => getComputedStyle(e).borderLeftColor + ' ' + getComputedStyle(e).borderLeftWidth")
                txt += " vs button-bg=" + await pg.evaluate("() => getComputedStyle(document.body).getPropertyValue('--button-bg')")
            except Exception as e:
                txt = "CARD NOT FOUND: " + str(e)[:100]
            for _ in range(3):
                await pg.keyboard.press("Escape")
            await pg.wait_for_timeout(800)
            await pg.screenshot(path=f"/out/a1-{u}.png")
            print(f"{u} card:", txt, "| pageerrors:", errs or "none")
            await ctx.close()
        await b.close()

asyncio.run(shots())
