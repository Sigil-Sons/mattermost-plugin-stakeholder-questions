"""SS-53 A1–A9 end-to-end on the throwaway MM (never prod). Real browsers as drew + scott, bot via REST.
Run: docker run --rm --network host -v /root/ssq-dogfood:/out --entrypoint python loot-runner:1.10.1 /out/e2e.py"""
import asyncio, json, sys, struct, urllib.request, uuid, zlib

MM = "http://127.0.0.1:8065"
P = MM + "/plugins/com.mysticspark.stakeholder-questions/api/v1"
creds = dict(l.strip().split("=", 1) for l in open("/out/creds") if "=" in l)
BOT = open("/out/bot.tok").read().strip()
RESULTS = []
CONSOLE = []
NET = []
PAGES = {}


def check(name, ok, detail=""):
    RESULTS.append((name, bool(ok), detail))
    print(("PASS " if ok else "FAIL ") + name + (f" | {detail}" if detail else ""), flush=True)


def png(w, h, rgb):
    raw = b"".join(b"\x00" + bytes(rgb) * w for _ in range(h))
    chunk = lambda t, d: struct.pack(">I", len(d)) + t + d + struct.pack(">I", zlib.crc32(t + d) & 0xffffffff)
    return b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", w, h, 8, 2, 0, 0, 0)) + \
        chunk(b"IDAT", zlib.compress(raw)) + chunk(b"IEND", b"")


def api(method, url, tok, body=None, raw=False):
    req = urllib.request.Request(url, method=method, data=json.dumps(body).encode() if body is not None else None,
                                 headers={"Authorization": "Bearer " + tok, "Content-Type": "application/json"})
    try:
        with urllib.request.urlopen(req) as r:
            b = r.read()
            return r.status, (b.decode() if raw else json.loads(b or b"{}"))
    except urllib.error.HTTPError as e:
        b = e.read()
        try:
            return e.code, json.loads(b or b"{}")
        except ValueError:
            return e.code, b.decode()


def upload(tok, channel_id, name, data):
    bnd = uuid.uuid4().hex
    body = (f"--{bnd}\r\nContent-Disposition: form-data; name=\"channel_id\"\r\n\r\n{channel_id}\r\n"
            f"--{bnd}\r\nContent-Disposition: form-data; name=\"files\"; filename=\"{name}\"\r\n"
            f"Content-Type: image/png\r\n\r\n").encode() + data + f"\r\n--{bnd}--\r\n".encode()
    req = urllib.request.Request(MM + "/api/v4/files", data=body, method="POST",
                                 headers={"Authorization": "Bearer " + tok, "Content-Type": f"multipart/form-data; boundary={bnd}"})
    with urllib.request.urlopen(req) as r:
        return json.loads(r.read())["file_infos"][0]["id"]


def login(u):
    req = urllib.request.Request(MM + "/api/v4/users/login", method="POST",
                                 data=json.dumps({"login_id": u, "password": creds[u]}).encode(),
                                 headers={"Content-Type": "application/json"})
    with urllib.request.urlopen(req) as r:
        return r.headers["Token"], json.loads(r.read())["id"]


open("/out/scott-sketch.png", "wb").write(png(120, 80, (230, 120, 40)))
ch = api("GET", MM + "/api/v4/teams/name/ssq/channels/name/office-hours", BOT)[1]
ctx_img = upload(BOT, ch["id"], "topology.png", png(320, 160, (40, 90, 200)))
mock_img = upload(BOT, ch["id"], "per-table-mock.png", png(160, 100, (60, 170, 90)))
code, pack = api("POST", P + "/packs", BOT, {
    "title": "StoryBard deployment topology", "summary": "E2E pack (throwaway MM)",
    "channel_id": ch["id"], "respondents": ["@drew", "scott"], "importance": "high",
    "questions": [
        {"prompt": "Where should the model bridge run?", "allow_other": True, "help_file_ids": [ctx_img],
         "help_markdown": "Context: the bridge talks to **Blade** for inference.",
         "options": [{"label": "Hermes LXC", "recommended": True, "description": "Next to the gateway"}, {"label": "Blade"}]},
        {"prompt": "Which audio features ship in v1?", "selection": "multi",
         "options": [{"label": "TTS narration"}, {"label": "Ambient music"}, {"label": "SFX"}]},
        {"prompt": "Subscription model?",
         "options": [{"label": "Per table", "image_file_id": mock_img}, {"label": "Per DM seat"}]},
    ]})
PID = pack.get("id")
check("A1 bot creates pack (201, 2 pending)", code == 201 and pack["min_respondents"] == 2, f"pack {PID} post {pack.get('post_id')}")
dtok, DID = login("drew")
stok, SID = login("scott")
_, cmd = api("POST", MM + "/api/v4/commands/execute", stok, {"channel_id": ch["id"], "command": "/ssq pending"})
check("/ssq pending lists pack for scott", "StoryBard deployment topology" in json.dumps(cmd), (cmd.get("text") or "")[:80])


async def browser_for(b, user):
    ctx = await b.new_context(viewport={"width": 1300, "height": 900})
    await ctx.add_init_script("try{localStorage.setItem('__landingPageSeen__','true')}catch(e){}")
    r = await ctx.request.post(MM + "/api/v4/users/login", data={"login_id": user, "password": creds[user]},
                               headers={"X-Requested-With": "XMLHttpRequest"})
    assert r.ok, await r.text()
    pg = await ctx.new_page()
    errs = []
    pg.on("pageerror", lambda e: errs.append(str(e)[:160] + " @ " + " <- ".join(
        l.strip()[:110] for l in (getattr(e, "stack", "") or "").splitlines()[1:4])))
    pg.on("response", lambda r: NET.append(f"{user} {r.request.method} {r.status} {r.url.split('/plugins/')[-1][:90]}") if ("stakeholder" in r.url or r.status >= 400) else None)
    pg.on("console", lambda m: CONSOLE.append(f"{user} [{m.type}] {m.text[:200]}") if m.type == "error" else None)
    PAGES[user] = pg
    await pg.goto(MM + "/ssq/channels/office-hours")
    await card(pg).wait_for(timeout=30000)
    await pg.keyboard.press("Escape")
    return ctx, pg, errs


def card(pg):
    return pg.locator(f'[data-testid="ssq-card"][data-pack-id="{PID}"]')


async def click(pg, testid):
    loc = pg.locator(f'[data-testid="{testid}"]').first
    if testid.startswith("ssq-option-"):
        loc = loc.locator("input")  # the row may hold a thumbnail link; select via the control itself
    await loc.click(force=True)


async def nxt(pg, expect_prefix):
    """Click Next and wait until the wizard shows the expected question (saves are async)."""
    await pg.locator('[data-testid="ssq-next"]').first.click(force=True)
    await pg.locator('[data-testid="ssq-prompt"]', has_text=expect_prefix).wait_for(timeout=15000)


async def imgs_ok(pg):
    return await pg.evaluate("() => [...document.querySelectorAll('[data-testid=ssq-img]')].map(i => i.complete && i.naturalWidth > 0)")


async def main():
    from playwright.async_api import async_playwright
    async with async_playwright() as p:
        b = await p.chromium.launch(args=["--no-sandbox"])
        try:
            await flow(b)
        except Exception:
            for u, pg in PAGES.items():
                try:
                    await pg.screenshot(path=f"/out/e2e-abort-{u}.png")
                    m = pg.locator('[data-testid="ssq-modal"]')
                    print(f"{u} modal:", (await m.inner_text())[-300:].replace("\n", " / ") if await m.count() else "none")
                except Exception as e2:
                    print("shot failed", u, e2)
            raise


async def flow(b):
    if True:
        dctx, dpg, derrs = await browser_for(b, "drew")
        # A2 drew answers all
        btn = card(dpg).locator('[data-testid="ssq-answer-btn"]')
        print("answer buttons:", await btn.count(), "| card:", (await card(dpg).inner_text()).replace("\n", " | "))
        await btn.click(force=True)
        try:
            await dpg.locator('[data-testid="ssq-wizard"]').wait_for(timeout=10000)
        except Exception:
            await dpg.screenshot(path="/out/e2e-debug.png")
            m = dpg.locator('[data-testid="ssq-modal"]')
            print("modal count:", await m.count(), "| text:", (await m.inner_text()) if await m.count() else "-", "| errs:", derrs)
            raise
        await dpg.wait_for_timeout(800)
        ok1 = await imgs_ok(dpg)
        check("A5 context image renders", ok1 and all(ok1), str(ok1))
        await dpg.screenshot(path="/out/e2e-1-wizard-q1.png")
        await click(dpg, "ssq-option-q1-hermes-lxc"); await nxt(dpg, "Which audio")
        await click(dpg, "ssq-option-q2-tts-narration"); await click(dpg, "ssq-option-q2-sfx"); await nxt(dpg, "Subscription")
        await dpg.wait_for_timeout(800)
        ok3 = await imgs_ok(dpg)
        q3 = await dpg.locator('[data-testid="ssq-prompt"]').inner_text()
        werr = await dpg.locator('[data-testid="ssq-wizard"]').inner_text()
        check("A5 option mockup image renders", q3.startswith("Subscription") and ok3 and all(ok3), f"{q3} {ok3}" + ("" if q3.startswith("Subscription") else " | wizard: " + werr[-160:].replace("\n", " / ")))
        await click(dpg, "ssq-option-q3-per-table"); await click(dpg, "ssq-submit")
        await dpg.locator('[data-testid="ssq-done"]').wait_for()
        await click(dpg, "ssq-close")
        await card(dpg).locator('[data-testid="ssq-progress"]', has_text="drew ✅").wait_for(timeout=15000)
        prog = await card(dpg).locator('[data-testid="ssq-progress"]').inner_text()
        check("A2 drew complete; card live-updates", "drew ✅" in prog and "scott ⏳" in prog, prog)
        _, dview = api("GET", f"{P}/packs/{PID}", dtok)
        check("A2 drew sees Scott empty (hidden by default)", set(dview["answers"]) == {DID}, str(list(dview["answers"])))
        check("A6 multi-select stores multiple option_ids", sorted(dview["answers"][DID]["q2"]["option_ids"]) == ["sfx", "tts-narration"],
              str(dview["answers"][DID]["q2"]["option_ids"]))
        # show others before scott answers
        await card(dpg).locator('[data-testid="ssq-answer-btn"]').click(force=True)
        await dpg.locator('[data-testid="ssq-wizard"]').wait_for()
        await click(dpg, "ssq-show-others")
        t = await dpg.locator('[data-testid="ssq-others"]').inner_text()
        check("Show others before Scott answers: nothing", "No one else has answered" in t, t)
        # A7 revise q3
        await nxt(dpg, "Which audio"); await nxt(dpg, "Subscription")
        await click(dpg, "ssq-option-q3-per-dm-seat"); await click(dpg, "ssq-submit")
        await dpg.locator('[data-testid="ssq-done"]').wait_for()
        await click(dpg, "ssq-close")
        _, dview = api("GET", f"{P}/packs/{PID}", dtok)
        check("A7 revise before lock updates answer", dview["answers"][DID]["q3"]["option_ids"] == ["per-dm-seat"],
              str(dview["answers"][DID]["q3"]))

        # A3 scott answers (Other + answer image)
        sctx, spg, serrs = await browser_for(b, "scott")
        await card(spg).locator('[data-testid="ssq-answer-btn"]').click(force=True)
        await spg.locator('[data-testid="ssq-wizard"]').wait_for()
        await click(spg, "ssq-other-q1")
        await spg.locator('[data-testid="ssq-other-text-q1"]').fill("Both, split by load")
        await spg.locator('[data-testid="ssq-file-input"]').set_input_files("/out/scott-sketch.png")
        await spg.locator('[data-testid="ssq-img"]').nth(1).wait_for(timeout=15000)
        await spg.screenshot(path="/out/e2e-2-scott-q1-other-image.png")
        await nxt(spg, "Which audio")
        await click(spg, "ssq-option-q2-ambient-music"); await nxt(spg, "Subscription")
        await click(spg, "ssq-option-q3-per-table"); await click(spg, "ssq-submit")
        await spg.locator('[data-testid="ssq-done"]').wait_for()
        # scott reviews and opts in to see drew
        await spg.get_by_text("Review my answers").click(force=True)
        await click(spg, "ssq-show-others")
        st = await spg.locator('[data-testid="ssq-others"]').inner_text()
        check("A3 scott sees drew's choice after opting in", "@drew" in st and "Hermes LXC" in st, st)
        await click(spg, "ssq-close")

        # drew sees scott after opting in; pack complete (A4)
        await dpg.locator(f'[data-pack-id="{PID}"][data-status="complete"]').wait_for(timeout=15000)
        dprog = await card(dpg).inner_text()
        check("A4 card shows 2/2 complete", "Quorum 2/2" in dprog and "scott ✅" in dprog and "Complete" in dprog, dprog.replace("\n", " | "))
        await card(dpg).locator('[data-testid="ssq-answer-btn"]').click(force=True)
        await dpg.locator('[data-testid="ssq-wizard"]').wait_for()
        await click(dpg, "ssq-show-others")
        await dpg.wait_for_timeout(800)
        dt = await dpg.locator('[data-testid="ssq-others"]').inner_text()
        ok_img = await imgs_ok(dpg)
        check("A3 drew sees scott's Other + image after opting in", "@scott" in dt and "Other: Both, split by load" in dt and ok_img and all(ok_img), dt)
        await dpg.screenshot(path="/out/e2e-3-drew-sees-scott.png")
        await click(dpg, "ssq-close")
        await card(dpg).locator('[data-testid="ssq-results-btn"]').click(force=True)
        await dpg.locator('[data-testid="ssq-results"]').wait_for()
        await dpg.wait_for_timeout(600)
        await dpg.screenshot(path="/out/e2e-4-results.png")
        rt = await dpg.locator('[data-testid="ssq-results"]').inner_text()
        check("Results view: histogram + names", "@drew" in rt and "@scott" in rt and "complete" in rt, rt[:120].replace("\n", " | "))
        await click(dpg, "ssq-close")
        await dpg.screenshot(path="/out/e2e-5-card-complete.png")

        _, md = api("GET", f"{P}/packs/{PID}/summary.md", dtok, raw=True)
        check("A4 summary.md lists decisions", "## Decisions" in md and "**Decision:** Hermes LXC" not in md and "Other, @scott: Both, split by load" in md,
              md.split("## Decisions")[-1][:160].replace("\n", " / "))
        _, thread = api("GET", f"{MM}/api/v4/posts/{pack['post_id']}/thread", BOT)
        done_msgs = [x["message"] for x in thread["posts"].values() if "Pack complete" in x.get("message", "")]
        check("Completion summary in thread @mentions creating bot", done_msgs and done_msgs[0].startswith("@sigil ✅ Pack complete"),
              (done_msgs or ["none"])[0][:70].replace("\n", " / "))
        c, e = api("POST", f"{P}/packs/{PID}/answers", dtok, {"question_id": "q1", "option_ids": ["blade"]})
        check("Answers frozen after quorum (409)", c == 409, f"{c} {e}")

        # lock path
        _, p2 = api("POST", P + "/packs", BOT, {"title": "Lock test", "channel_id": ch["id"], "respondents": ["drew", "scott"],
                                                "questions": [{"prompt": "x?", "options": [{"label": "a"}, {"label": "b"}]}]})
        c1, _ = api("POST", f"{P}/packs/{p2['id']}/lock", dtok)
        c2, l2 = api("POST", f"{P}/packs/{p2['id']}/lock", BOT)
        c3, _ = api("POST", f"{P}/packs/{p2['id']}/answers", dtok, {"question_id": "q1", "option_ids": ["a"]})
        check("Lock: respondent 403, author 200 locked, then answers 409", (c1, c2, l2.get("status"), c3) == (403, 200, "locked", 409), f"{c1},{c2},{l2.get('status')},{c3}")
        def status(url, tok):
            try:
                with urllib.request.urlopen(urllib.request.Request(url, headers={"Authorization": "Bearer " + tok})) as r:
                    return r.status, r.headers.get("Content-Type")
            except urllib.error.HTTPError as e:
                return e.code, None
        c4, ct = status(f"{P}/packs/{PID}/files/{ctx_img}", dtok)
        c5, _ = status(f"{P}/packs/{PID}/files/notreferenced123", dtok)
        c4 = c4 if ct == "image/png" else f"{c4}/{ct}"
        check("File proxy serves referenced images only", c4 == 200 and c5 == 404, f"{c4},{c5}")

        # any bot: sons posts via the portable CLI; quorum gets sons @mentioned
        import subprocess
        sons_env = {"MATTERMOST_URL": MM, "MATTERMOST_TOKEN": open("/out/sons.tok").read().strip(), "PATH": "/usr/bin:/bin"}
        json.dump({"title": "Sons asks", "respondents": ["drew", "scott"], "questions": [
            {"prompt": "Print-on-demand provider?", "options": [{"label": "Printful", "recommended": True}, {"label": "Gelato"}]}]},
            open("/out/sons-pack.json", "w"))
        cp = subprocess.run([sys.executable, "/out/ssq.py", "create", "/out/sons-pack.json", "--channel", "ssq/office-hours"],
                            capture_output=True, text=True, env=sons_env)
        sons_pack = json.loads(cp.stdout or "{}") if cp.returncode == 0 else {}
        check("Any bot: @sons creates via skill CLI", bool(sons_pack.get("pack_id")), (cp.stdout or cp.stderr).strip()[:100])
        if sons_pack.get("pack_id"):
            for t in (dtok, stok):
                api("POST", f"{P}/packs/{sons_pack['pack_id']}/answers", t, {"question_id": "q1", "option_ids": ["printful"]})
            _, th = api("GET", f"{MM}/api/v4/posts/{sons_pack['post_id']}/thread", BOT)
            msgs = [x["message"] for x in th["posts"].values() if "Pack complete" in x.get("message", "")]
            check("Any bot: completion @mentions @sons (not @sigil)", msgs and msgs[0].startswith("@sons "), (msgs or ["none"])[0][:50])
            cs = subprocess.run([sys.executable, "/out/ssq.py", "summary", sons_pack["pack_id"]], capture_output=True, text=True, env=sons_env)
            check("Any bot: skill CLI reads summary", "**Decision:** Printful (all 2 agree)" in cs.stdout, cs.stdout[-80:].replace("\n", " / "))

        # human: drew creates a pack with /ssq new (interactive dialog), mentioning scott
        before = len(api("GET", f"{MM}/api/v4/channels/{ch['id']}/posts?per_page=30", BOT)[1]["order"])
        box = dpg.locator("#post_textbox")
        await box.click(force=True)
        await box.fill("/ssq new")
        await dpg.keyboard.press("Enter")
        dlg = dpg.locator("#interactiveDialogModal, [role=dialog]:has-text('New stakeholder pack')").first
        try:
            await dlg.wait_for(timeout=15000)
            await dpg.wait_for_timeout(1200)  # let the modal fade-in finish before the proof shot
            await dpg.screenshot(path="/out/e2e-6-ssq-new-dialog.png")
            await dlg.locator("input#title, input[id$='title']").first.fill("Human-made pack")
            await dlg.locator("input#respondents, input[id$='respondents']").first.fill("@scott")
            await dlg.locator("textarea#questions, textarea[id$='questions']").first.fill(
                "Logo direction?\n- Seal (recommended) :: classic\n- Wordmark\n- Other")
            await dlg.locator("#interactiveDialogSubmit, button:has-text('Post pack')").first.click(force=True)
            human = dpg.locator('[data-testid="ssq-card"]', has_text="Human-made pack").last
            await human.wait_for(timeout=15000)
            ht = (await human.inner_text()).replace("\n", " | ")
            check("Any participant: /ssq new dialog creates a pack", "created by @drew" in ht and "@scott" in ht, ht[:140])
        except Exception as ex:
            await dpg.screenshot(path="/out/e2e-6-ssq-new-FAIL.png")
            check("Any participant: /ssq new dialog creates a pack", False, str(ex).splitlines()[0][:120])

        for who, errs in (("drew", derrs), ("scott", serrs)):
            ours = [e for e in errs if "stakeholder-questions" in e or "render is not a function" in e]
            for e in errs:
                print(f"INFO page error ({who}): {e[:600]}")
            check(f"A9 React 19: no page errors from this plugin ({who})", not ours, str(ours) if ours else f"{len(errs)} non-plugin error(s)")
        await b.close()

try:
    asyncio.run(main())
except Exception as ex:
    print("ABORT:", str(ex).splitlines()[0])
    import traceback; print("at:", traceback.format_exc().strip().splitlines()[-4].strip())
    print("console errors:", *CONSOLE[-12:], sep="\n  ")
    print("network:", *NET[-20:], sep="\n  ")
fails = [r for r in RESULTS if not r[1]]
print(f"\nSUMMARY {len(RESULTS) - len(fails)}/{len(RESULTS)} passed")
json.dump(RESULTS, open("/out/e2e-results.json", "w"), indent=1)
