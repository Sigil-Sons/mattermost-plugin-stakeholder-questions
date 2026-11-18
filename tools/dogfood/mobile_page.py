"""Phone-browser path (Pixel 7 emulation) for the standalone answer page, on the throwaway MM only.
Logged-out tap on the post's link -> MM login -> back to the page -> answer -> server stores it."""
import asyncio, json, re, urllib.request

MM = "http://127.0.0.1:8065"
P = MM + "/plugins/com.mysticspark.stakeholder-questions/api/v1"
creds = dict(l.strip().split("=", 1) for l in open("/out/creds") if "=" in l)
BOT = open("/out/bot.tok").read().strip()
OK = []
CON = []


def check(name, ok, detail=""):
    OK.append(bool(ok))
    print(("PASS " if ok else "FAIL ") + name + (f" | {detail}" if detail else ""), flush=True)


def api(method, url, body=None):
    r = urllib.request.Request(url, method=method, data=json.dumps(body).encode() if body is not None else None,
                               headers={"Authorization": "Bearer " + BOT, "Content-Type": "application/json"})
    with urllib.request.urlopen(r) as resp:
        return json.loads(resp.read() or b"{}")


ch = api("GET", MM + "/api/v4/teams/name/ssq/channels/name/office-hours")
pack = api("POST", P + "/packs", {"title": "Phone page test", "summary": "answer from the standalone page",
                                  "channel_id": ch["id"], "respondents": ["drew", "scott"], "questions": [
    {"prompt": "Works from the app link?", "allow_other": True, "help_markdown": "Tap through from the Android app.",
     "options": [{"label": "Yes", "recommended": True, "description": "smooth"}, {"label": "No"}]},
    {"prompt": "Which parts?", "selection": "multi", "options": [{"label": "Questions"}, {"label": "Results"}, {"label": "Login"}]}]})
PID = pack["id"]
msg = api("GET", f"{MM}/api/v4/posts/{pack['post_id']}")["message"]
link = re.search(r"\[Answer in browser\]\(([^)]+)\)", msg)
check("Fallback text lists questions", "1. Works from the app link? _(pick one)_: Yes (recommended) · No · Other" in msg
      and "2. Which parts? _(pick any)_" in msg, msg.splitlines()[1][:90])
check("Fallback text has answer link", bool(link), link.group(1) if link else msg[-80:])


async def main():
    from playwright.async_api import async_playwright
    async with async_playwright() as p:
        b = await p.chromium.launch(args=["--no-sandbox"])
        ctx = await b.new_context(**p.devices["Pixel 7"])
        await ctx.add_init_script("try{localStorage.setItem('__landingPageSeen__','true')}catch(e){}")
        pg = await ctx.new_page()
        errs = []
        pg.on("pageerror", lambda e: errs.append(str(e)[:150]))
        pg.on("console", lambda m: CON.append(f"[{m.type}] {m.text[:200]}"))
        pg.on("response", lambda r: CON.append(f"{r.status} {r.url.replace(MM, '')[:100]}") if "answer" in r.url or r.status >= 400 else None)
        await pg.goto(link.group(1))
        await pg.wait_for_url(re.compile(r"/login"), timeout=20000)
        check("Logged out -> Mattermost login with redirect back", "redirect_to" in pg.url, pg.url.replace(MM, "")[:90])
        await pg.locator("#input_loginId").fill("drew")
        await pg.locator("#input_password-input").fill(creds["drew"])
        await pg.locator("#saveSetting").click()
        await pg.wait_for_url(re.compile(r"/answer/" + PID), timeout=30000)
        try:
            await pg.locator('[data-testid="m-prompt"]').wait_for(timeout=15000)
        except Exception:
            print("DEBUG url:", pg.url, "| body:", (await pg.locator("body").inner_text())[:300], "| errs:", errs, "| console:", CON[-6:])
            raise
        check("After login lands back on the answer page", True, pg.url.replace(MM, "")[:90])
        await pg.screenshot(path="/out/m4-page-q1.png")
        await pg.locator('[data-testid="m-opt-yes"] input').tap()
        await pg.locator('[data-testid="m-show-others"]').tap()
        await pg.get_by_text("No one else has answered this yet.").wait_for(timeout=10000)
        check("Show others (nothing yet) works on the page", True)
        await pg.locator('[data-testid="m-next"]').tap()
        await pg.locator('[data-testid="m-prompt"]', has_text="Which parts?").wait_for(timeout=10000)
        await pg.locator('[data-testid="m-opt-questions"] input').tap()
        await pg.locator('[data-testid="m-opt-results"] input').tap()
        await pg.locator('[data-testid="m-submit"]').tap()
        await pg.locator('[data-testid="m-done"]', has_text="All answered").wait_for(timeout=10000)
        await pg.locator('[data-testid="m-results"] h2').first.wait_for(timeout=10000)
        await pg.screenshot(path="/out/m5-page-done.png", full_page=True)
        got = api("GET", f"{P}/packs/{PID}")["answers"]
        a = list(got.values())[0] if got else {}
        check("Answers stored from the page", a.get("q1", {}).get("option_ids") == ["yes"]
              and sorted(a.get("q2", {}).get("option_ids", [])) == ["questions", "results"], str({k: v["option_ids"] for k, v in a.items()}))
        post = api("GET", f"{MM}/api/v4/posts/{pack['post_id']}")
        check("Card progress updated", "drew" in post["props"].get("ssq_completed", []), str(post["props"].get("ssq_completed")))
        check("No page errors on the standalone page", not errs, str(errs))
        await b.close()

asyncio.run(main())
print(f"SUMMARY {sum(OK)}/{len(OK)} passed")
