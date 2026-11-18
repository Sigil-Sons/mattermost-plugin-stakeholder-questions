// Standalone phone-friendly answer page for Stakeholder Questions (no Mattermost web app needed).
// Served by the plugin; uses the normal Mattermost browser session (cookie + CSRF header).
(function () {
  "use strict";
  var parts = location.pathname.split("/");
  var packId = decodeURIComponent(parts[parts.length - 1] || "");
  var base = location.pathname.slice(0, location.pathname.indexOf("/answer/")) + "/api/v1";
  var root = location.pathname.slice(0, location.pathname.indexOf("/plugins/"));
  var app = document.getElementById("app");
  var me = "", pack = null, idx = 0, drafts = {}, othersPack = null, showOthers = false, busy = false, err = "", done = false;

  function csrf() { var m = document.cookie.match(/(?:^|; )MMCSRF=([^;]+)/); return m ? decodeURIComponent(m[1]) : ""; }
  function call(method, url, body) {
    var h = { "X-Requested-With": "XMLHttpRequest", "X-CSRF-Token": csrf() };
    if (body !== undefined) h["Content-Type"] = "application/json";
    return fetch(url, { method: method, credentials: "include", headers: h, body: body === undefined ? undefined : JSON.stringify(body) })
      .then(function (r) {
        if (r.status === 401) { location.href = root + "/login?redirect_to=" + encodeURIComponent(location.pathname); throw new Error("Please log in"); }
        return r.text().then(function (t) {
          var d = {}; try { d = t ? JSON.parse(t) : {}; } catch (e) { d = { error: t }; }
          if (!r.ok) throw new Error(d.error || d.message || ("HTTP " + r.status));
          return d;
        });
      });
  }
  function el(tag, attrs, kids) {
    var n = document.createElement(tag);
    Object.keys(attrs || {}).forEach(function (k) {
      if (k === "text") n.textContent = attrs[k];
      else if (k.slice(0, 2) === "on") n.addEventListener(k.slice(2), attrs[k]);
      else if (k === "checked" || k === "disabled") n[k] = !!attrs[k];
      else n.setAttribute(k, attrs[k]);
    });
    (kids || []).forEach(function (c) { if (c != null) n.appendChild(typeof c === "string" ? document.createTextNode(c) : c); });
    return n;
  }
  function fileUrl(f, others) { return base + "/packs/" + packId + "/files/" + f + (others ? "?show_others=1" : ""); }
  function uname(uid, p) { var i = p.respondent_user_ids.indexOf(uid); return "@" + (i >= 0 ? p.respondent_usernames[i] : uid.slice(0, 6)); }
  function labels(q, a) {
    if (a.skipped) return "Skipped";
    var ls = a.option_ids.map(function (id) { var o = q.options.filter(function (x) { return x.id === id; })[0]; return o ? o.label : id; });
    if (a.other_text) ls.push("Other: " + a.other_text);
    return ls.join(", ");
  }
  function draftFor(q) {
    if (!drafts[q.id]) {
      var a = (pack.answers[me] || {})[q.id];
      drafts[q.id] = { ids: a ? a.option_ids.slice() : [], other: a && a.other_text ? a.other_text : "", otherOn: !!(a && a.other_text) };
    }
    return drafts[q.id];
  }

  function render() {
    app.innerHTML = "";
    if (!pack) { app.appendChild(el("p", { "class": err ? "err" : "muted", text: err || "Loading…" })); return; }
    app.appendChild(el("div", { "class": "muted small", text: "📋 Stakeholder pack" }));
    app.appendChild(el("h1", { text: pack.title }));
    if (pack.summary) app.appendChild(el("div", { "class": "muted", text: pack.summary }));
    var isResp = pack.respondent_user_ids.indexOf(me) >= 0;
    var editable = isResp && pack.status === "open";
    if (done || !editable) { renderResults(isResp, editable); return; }

    var q = pack.questions[idx], d = draftFor(q), n = pack.questions.length;
    var answered = Object.keys(pack.answers[me] || {}).length;
    app.appendChild(el("div", { "class": "eyebrow", text: "Q " + (idx + 1) + " of " + n + " · " + answered + "/" + n + " answered" + (q.selection === "multi" ? " · pick any" : "") }));
    app.appendChild(el("h2", { "data-testid": "m-prompt", text: q.prompt }));
    if (q.help_markdown) app.appendChild(el("div", { "class": "help", text: q.help_markdown }));
    (q.help_file_ids || []).forEach(function (f) { app.appendChild(el("img", { "class": "thumb", src: fileUrl(f), alt: "context" })); });
    var type = q.selection === "single" ? "radio" : "checkbox";
    q.options.forEach(function (o) {
      var on = d.ids.indexOf(o.id) >= 0 && !(type === "radio" && d.otherOn);
      var img = o.image_file_id ? fileUrl(o.image_file_id) : (o.image_url || "");
      app.appendChild(el("label", { "class": "opt" + (on ? " on" : ""), "data-testid": "m-opt-" + o.id }, [
        el("input", { type: type, name: "q", checked: on, onchange: function () {
          if (type === "radio") { d.ids = [o.id]; d.otherOn = false; }
          else { var i = d.ids.indexOf(o.id); if (i >= 0) d.ids.splice(i, 1); else d.ids.push(o.id); }
          render();
        } }),
        el("div", {}, [
          el("b", {}, [o.label, o.recommended ? el("span", { "class": "pill", text: "Recommended" }) : null]),
          o.description ? el("span", { "class": "muted small", text: o.description }) : null,
          img ? el("img", { "class": "thumb", src: img, alt: o.label }) : null
        ])
      ]));
    });
    if (q.allow_other) {
      var box = el("label", { "class": "opt" + (d.otherOn ? " on" : ""), "data-testid": "m-opt-other" }, [
        el("input", { type: type, name: "q", checked: d.otherOn, onchange: function () {
          if (type === "radio") { d.otherOn = true; d.ids = []; } else d.otherOn = !d.otherOn;
          render();
        } }),
        el("div", { style: "flex:1" }, [el("b", { text: "Other" }),
          d.otherOn ? el("textarea", { "data-testid": "m-other-text", maxlength: "2000", placeholder: "Your answer", oninput: function (e) { d.other = e.target.value; } }) : null])
      ]);
      app.appendChild(box);
      var ta = box.querySelector("textarea"); if (ta) ta.value = d.other;
    }
    var oth = el("div", { "class": "others" }, [el("button", { "class": "link", "data-testid": "m-show-others", text: showOthers ? "Hide others’ answers" : "Show others’ answers", onclick: toggleOthers })]);
    if (showOthers && othersPack) {
      var any = false;
      Object.keys(othersPack.answers || {}).forEach(function (uid) {
        var a = othersPack.answers[uid][q.id];
        if (uid === me || !a) return;
        any = true;
        oth.appendChild(el("div", { "class": "small" }, [el("b", { text: uname(uid, othersPack) + " " }), labels(q, a)]));
        (a.image_file_ids || []).forEach(function (f) { oth.appendChild(el("img", { "class": "thumb", src: fileUrl(f, true), alt: "answer image" })); });
      });
      if (!any) oth.appendChild(el("div", { "class": "muted small", text: "No one else has answered this yet." }));
    }
    app.appendChild(oth);
    if (err) app.appendChild(el("div", { "class": "err", text: err }));
    var bar = el("div", { "class": "bar" });
    if (pack.allow_skip) bar.appendChild(el("button", { disabled: busy, text: "Skip", onclick: function () { next(true); } }));
    bar.appendChild(el("button", { disabled: busy || idx === 0, text: "Back", onclick: function () { idx--; showOthers = false; err = ""; render(); } }));
    bar.appendChild(el("button", { "class": "primary", "data-testid": idx === n - 1 ? "m-submit" : "m-next", disabled: busy,
      text: idx === n - 1 ? "Submit" : "Next", onclick: function () { next(false); } }));
    app.appendChild(bar);
  }

  function toggleOthers() {
    if (showOthers) { showOthers = false; render(); return; }
    call("GET", base + "/packs/" + packId + "?show_others=1").then(function (p) { othersPack = p; showOthers = true; render(); })
      .catch(function (e) { err = e.message; render(); });
  }

  function next(skip) {
    var q = pack.questions[idx], d = draftFor(q);
    busy = true; err = ""; render();
    var body = { question_id: q.id, option_ids: d.otherOn && q.selection === "single" ? [] : d.ids,
      other_text: d.otherOn && d.other.trim() ? d.other.trim() : null, image_file_ids: ((pack.answers[me] || {})[q.id] || {}).image_file_ids || [], skip: !!skip };
    call("POST", base + "/packs/" + packId + "/answers", body).then(function (p) {
      pack = p; busy = false; showOthers = false;
      if (idx < pack.questions.length - 1) idx++; else done = true;
      render(); window.scrollTo(0, 0);
    }).catch(function (e) { busy = false; err = e.message; render(); });
  }

  function renderResults(isResp, editable) {
    var mine = pack.answers[me] || {};
    var complete = pack.completed_user_ids.indexOf(me) >= 0;
    if (isResp) app.appendChild(el("p", { "class": complete ? "ok" : "muted", "data-testid": "m-done",
      text: complete ? "All answered ✅" : (editable ? "Saved." : "This pack is " + pack.status + ".") }));
    else app.appendChild(el("p", { "class": "muted", text: "You are not a respondent on this pack." }));
    var res = el("div", { "data-testid": "m-results" });
    app.appendChild(res);
    call("GET", base + "/packs/" + packId + "?show_others=1").then(function (p) {
      p.questions.forEach(function (q, i) {
        res.appendChild(el("h2", { text: (i + 1) + ". " + q.prompt }));
        var ans = p.respondent_user_ids.map(function (u) { return { u: u, a: (p.answers[u] || {})[q.id] }; }).filter(function (x) { return x.a && !x.a.skipped; });
        var total = ans.length || 1;
        q.options.forEach(function (o) {
          var who = ans.filter(function (x) { return x.a.option_ids.indexOf(o.id) >= 0; }).map(function (x) { return uname(x.u, p); });
          res.appendChild(el("div", { "class": "row" }, [el("span", { text: o.label }), el("span", { "class": "muted", text: who.length + (who.length ? " · " + who.join(", ") : "") })]));
          res.appendChild(el("div", { "class": "hist" }, [el("div", { style: "width:" + Math.round(who.length / total * 100) + "%" })]));
        });
        ans.filter(function (x) { return x.a.other_text; }).forEach(function (x) { res.appendChild(el("div", { "class": "small", text: "Other · " + uname(x.u, p) + ": " + x.a.other_text })); });
        if (!ans.length) res.appendChild(el("div", { "class": "muted small", text: "No answers yet." }));
      });
    }).catch(function (e) { res.appendChild(el("div", { "class": "err", text: e.message })); });
    var bar = el("div", { "class": "bar" });
    if (editable) bar.appendChild(el("button", { text: "Review my answers", onclick: function () { done = false; idx = 0; render(); } }));
    bar.appendChild(el("button", { "class": "primary", text: "Back to Mattermost", onclick: function () { location.href = root + "/"; } }));
    app.appendChild(bar);
    void mine;
  }

  call("GET", root + "/api/v4/users/me").then(function (u) {
    me = u.id;
    return call("GET", base + "/packs/" + packId);
  }).then(function (p) {
    pack = p;
    var first = p.questions.findIndex(function (q) { return !(p.answers[me] || {})[q.id]; });
    idx = first >= 0 ? first : 0;
    done = p.respondent_user_ids.indexOf(me) >= 0 && first < 0;
    render();
  }).catch(function (e) { err = e.message; render(); });
})();
