#!/usr/bin/env python3
"""Stakeholder question packs CLI (stdlib only). Env: MATTERMOST_URL, MATTERMOST_TOKEN.

  ssq.py create pack.json --channel TEAM/CHANNEL   -> prints pack_id + post_id
  ssq.py get PACK_ID                               -> pack JSON (as author: every answer)
  ssq.py summary PACK_ID                           -> markdown decision log
  ssq.py wait PACK_ID [--timeout S]                -> polls until complete/locked/expired
  ssq.py upload TEAM/CHANNEL IMAGE                 -> prints file id for help_file_ids / image_file_id
"""
import argparse, json, mimetypes, os, sys, time, urllib.error, urllib.request, uuid

PLUGIN = "/plugins/com.mysticspark.stakeholder-questions/api/v1"


def env():
    url, tok = os.environ.get("MATTERMOST_URL", "").rstrip("/"), os.environ.get("MATTERMOST_TOKEN", "")
    if not url or not tok:
        sys.exit("set MATTERMOST_URL and MATTERMOST_TOKEN")
    return url, tok


def req(method, path, body=None, raw=False, ctype="application/json", data=None):
    url, tok = env()
    if body is not None:
        data = json.dumps(body).encode()
    r = urllib.request.Request(url + path, data=data, method=method,
                               headers={"Authorization": "Bearer " + tok, "Content-Type": ctype})
    try:
        with urllib.request.urlopen(r, timeout=60) as resp:
            b = resp.read()
            return b.decode() if raw else json.loads(b or b"{}")
    except urllib.error.HTTPError as e:
        msg = e.read().decode(errors="replace")
        try:
            msg = json.loads(msg).get("error") or json.loads(msg).get("message") or msg
        except ValueError:
            pass
        sys.exit(f"HTTP {e.code} {method} {path.split('?')[0]}: {msg}")


def channel_id(ref):
    if "/" not in ref:
        return ref  # already an id
    team, ch = ref.split("/", 1)
    return req("GET", f"/api/v4/teams/name/{team}/channels/name/{ch}")["id"]


def cmd_create(a):
    pack = json.load(open(a.file))
    if a.channel:
        pack["channel_id"] = channel_id(a.channel)
    if not pack.get("channel_id"):
        sys.exit("give --channel TEAM/CHANNEL or channel_id in the file")
    out = req("POST", PLUGIN + "/packs", pack)
    print(json.dumps({"pack_id": out["id"], "post_id": out["post_id"], "min_respondents": out["min_respondents"],
                      "questions": [q["id"] for q in out["questions"]]}))


def cmd_get(a):
    print(json.dumps(req("GET", f"{PLUGIN}/packs/{a.pack_id}"), indent=2))


def cmd_summary(a):
    print(req("GET", f"{PLUGIN}/packs/{a.pack_id}/summary.md", raw=True))


def cmd_wait(a):
    end = time.time() + a.timeout
    while True:
        p = req("GET", f"{PLUGIN}/packs/{a.pack_id}")
        if p["status"] != "open":
            print(json.dumps({"status": p["status"], "completed": len(p["completed_user_ids"])}))
            return
        if time.time() > end:
            sys.exit(f"still open after {a.timeout}s ({len(p['completed_user_ids'])}/{p['min_respondents']} done)")
        time.sleep(a.interval)


def cmd_upload(a):
    cid = channel_id(a.channel)
    bnd = uuid.uuid4().hex
    name = os.path.basename(a.image)
    mt = mimetypes.guess_type(name)[0] or "application/octet-stream"
    body = (f"--{bnd}\r\nContent-Disposition: form-data; name=\"channel_id\"\r\n\r\n{cid}\r\n"
            f"--{bnd}\r\nContent-Disposition: form-data; name=\"files\"; filename=\"{name}\"\r\nContent-Type: {mt}\r\n\r\n").encode() \
        + open(a.image, "rb").read() + f"\r\n--{bnd}--\r\n".encode()
    out = req("POST", "/api/v4/files", data=body, ctype=f"multipart/form-data; boundary={bnd}")
    print(out["file_infos"][0]["id"])


def main():
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    sp = ap.add_subparsers(dest="cmd", required=True)
    c = sp.add_parser("create"); c.add_argument("file"); c.add_argument("--channel"); c.set_defaults(f=cmd_create)
    g = sp.add_parser("get"); g.add_argument("pack_id"); g.set_defaults(f=cmd_get)
    s = sp.add_parser("summary"); s.add_argument("pack_id"); s.set_defaults(f=cmd_summary)
    wt = sp.add_parser("wait"); wt.add_argument("pack_id"); wt.add_argument("--timeout", type=int, default=600)
    wt.add_argument("--interval", type=int, default=15); wt.set_defaults(f=cmd_wait)
    u = sp.add_parser("upload"); u.add_argument("channel"); u.add_argument("image"); u.set_defaults(f=cmd_upload)
    a = ap.parse_args()
    a.f(a)


if __name__ == "__main__":
    main()
