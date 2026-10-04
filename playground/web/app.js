// monadz playground. Plain ES modules, no dependencies.
import { snippets } from "./snippets.js";

// ---------- small DOM helpers ----------

function h(tag, attrs = {}, ...children) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs || {})) {
    if (v === null || v === undefined || v === false) continue;
    if (k === "class") el.className = v;
    else if (k === "style") el.setAttribute("style", v);
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (k === "text") el.textContent = v;
    else el.setAttribute(k, v === true ? "" : v);
  }
  append(el, children);
  return el;
}

function append(el, children) {
  for (const c of children.flat(Infinity)) {
    if (c === null || c === undefined || c === false) continue;
    el.append(c instanceof Node ? c : document.createTextNode(String(c)));
  }
}

function clear(el, ...children) {
  el.replaceChildren();
  append(el, children);
}

function debounce(fn, ms) {
  let t;
  return (...args) => {
    clearTimeout(t);
    t = setTimeout(() => fn(...args), ms);
  };
}

const getPath = (obj, path) => path.split(".").reduce((o, k) => (o == null ? o : o[k]), obj);
function setPath(obj, path, value) {
  const keys = path.split(".");
  const last = keys.pop();
  const target = keys.reduce((o, k) => o[k], obj);
  target[last] = value;
}

let uid = 0;
const nextId = (p) => `${p}-${++uid}`;

const ms = (n) => `${Math.round(n)} ms`;

// ---------- the WASM bridge ----------

let ready = false;
const readyWaiters = [];

function callSync(name, input) {
  const raw = window.monadz[name](JSON.stringify(input));
  return JSON.parse(raw);
}

async function callAsync(name, input, onEvent) {
  const raw = await window.monadz[name](JSON.stringify(input), (e) => onEvent && onEvent(JSON.parse(e)));
  return JSON.parse(raw);
}

function bridgeError(msg) {
  return h("div", { class: "answer err", role: "alert" },
    h("div", { class: "answer-title" }, "The playground got an error from the bridge"),
    h("pre", {}, msg));
}

// ---------- form builder ----------

function buildForm(fields, state, onChange) {
  const syncers = [];
  const errorSlots = {};
  const root = h("div", {});

  function textField(f) {
    const id = nextId("in");
    const errId = `${id}-err`;
    const input = h("input", {
      id, type: f.inputType || "text", autocomplete: "off", spellcheck: "false",
      "aria-describedby": f.hint ? `${id}-hint` : null,
    });
    input.addEventListener("input", () => { setPath(state, f.key, input.value); onChange(); });
    syncers.push(() => { input.value = getPath(state, f.key) ?? ""; });
    const err = h("span", { class: "field-error", id: errId });
    errorSlots[f.key] = { input, err, hintId: f.hint ? `${id}-hint` : null };
    return h("div", { class: "field" },
      h("label", { class: "field-label", for: id }, f.label),
      input,
      f.hint && h("span", { class: "field-hint", id: `${id}-hint` }, f.hint),
      err);
  }

  function textareaField(f) {
    const id = nextId("in");
    const input = h("textarea", { id, spellcheck: "false", rows: f.rows || 6, "aria-describedby": f.hint ? `${id}-hint` : null });
    input.addEventListener("input", () => { setPath(state, f.key, input.value); onChange(); });
    syncers.push(() => { input.value = getPath(state, f.key) ?? ""; });
    errorSlots[f.key] = { input, err: h("span"), hintId: null };
    return h("div", { class: "field" },
      h("label", { class: "field-label", for: id }, f.label),
      input,
      f.hint && h("span", { class: "field-hint", id: `${id}-hint` }, f.hint));
  }

  function switchField(f) {
    const input = h("input", { type: "checkbox", role: "switch", class: f.okStyle ? "ok-switch" : null });
    input.addEventListener("change", () => { setPath(state, f.key, input.checked); onChange(); });
    syncers.push(() => { input.checked = !!getPath(state, f.key); });
    return h("label", { class: "switch" },
      h("span", { class: "switch-text" }, f.label, f.small && h("small", {}, f.small)),
      input);
  }

  function segField(f) {
    const name = nextId("seg");
    const inputs = [];
    const opts = f.options.map((o) => {
      const input = h("input", { type: "radio", name, value: o.value });
      input.addEventListener("change", () => { if (input.checked) { setPath(state, f.key, coerce(f, o.value)); onChange(); } });
      inputs.push(input);
      return h("label", {}, input, h("span", {}, o.label));
    });
    syncers.push(() => { const v = String(getPath(state, f.key)); for (const i of inputs) i.checked = i.value === v; });
    return h("fieldset", { class: "segmented" },
      h("legend", { class: f.hideLegend ? "sr-only" : null }, f.label),
      h("div", { class: `seg-options ${f.cls || ""}` }, opts));
  }

  function modesField(f) {
    const name = nextId("mode");
    const inputs = [];
    const opts = f.options.map((o) => {
      const input = h("input", { type: "radio", name, value: o.value });
      input.addEventListener("change", () => { if (input.checked) { setPath(state, f.key, o.value); onChange(); } });
      inputs.push(input);
      return h("label", { class: "mode-card" }, input, h("span", {}, h("b", {}, o.code), o.desc));
    });
    syncers.push(() => { const v = getPath(state, f.key); for (const i of inputs) i.checked = i.value === v; });
    return h("fieldset", { class: "segmented" }, h("legend", {}, f.label), h("div", { class: "mode-cards" }, opts));
  }

  function rangeField(f) {
    const id = nextId("in");
    const input = h("input", { id, type: "range", min: f.min, max: f.max, step: f.step || 1 });
    const out = h("output", { for: id });
    const show = (v) => { out.textContent = f.format ? f.format(v) : v; };
    input.addEventListener("input", () => { const v = Number(input.value); setPath(state, f.key, v); show(v); onChange(); });
    syncers.push(() => { const v = getPath(state, f.key); input.value = v; show(v); });
    return h("div", { class: "field" },
      h("label", { class: "field-label", for: id }, f.label),
      h("div", { class: "range-row" }, input, out),
      f.hint && h("span", { class: "field-hint" }, f.hint));
  }

  function groupField(f) {
    return h("fieldset", { class: "task-input" }, h("legend", {}, f.legend), f.fields.map(build));
  }

  function build(f) {
    switch (f.type) {
      case "text": return textField(f);
      case "textarea": return textareaField(f);
      case "switch": return switchField(f);
      case "seg": return segField(f);
      case "modes": return modesField(f);
      case "range": return rangeField(f);
      case "group": return groupField(f);
      case "node": return f.node;
      default: throw new Error(`unknown field type ${f.type}`);
    }
  }

  append(root, fields.map(build));
  const sync = () => syncers.forEach((s) => s());
  sync();

  function setErrors(byKey) {
    for (const [key, slot] of Object.entries(errorSlots)) {
      const msgs = byKey[key] || [];
      slot.err.textContent = msgs.join("; ");
      if (msgs.length) {
        slot.input.setAttribute("aria-invalid", "true");
        slot.input.setAttribute("aria-describedby", [slot.hintId, slot.err.id].filter(Boolean).join(" "));
      } else {
        slot.input.removeAttribute("aria-invalid");
        if (slot.hintId) slot.input.setAttribute("aria-describedby", slot.hintId);
        else slot.input.removeAttribute("aria-describedby");
      }
    }
  }
  return { el: root, sync, setErrors };
}

function coerce(f, v) {
  return f.number ? Number(v) : v === "true" ? true : v === "false" ? false : v;
}

// ---------- the railway ----------

const GOOD = new Set(["ok", "some", "right"]);
const BAD = new Set(["err", "none", "left"]);

function legend(goodLabel, badLabel) {
  return h("div", { class: "legend", "aria-hidden": "true" },
    h("span", {}, h("i", { class: "l-ok" }), goodLabel),
    h("span", {}, h("i", { class: "l-err" }), badLabel),
    h("span", {}, h("i", { class: "l-skip" }), "skipped"));
}

// renderRail draws the steps on two tracks. labels maps a track to the badge
// text. opts.crash stops the bottom track at the panic.
function renderRail(steps, labels, prevSig, opts = {}) {
  const n = steps.length;
  const sig = steps.map((s) => s.track).join(",");
  const rail = h("ol", { class: `rail${sig !== prevSig ? " animate" : ""}`, style: `--n:${n}`, "aria-label": "Pipeline steps" });
  const stop = steps.findIndex((s) => BAD.has(s.track) || s.track === "panic");

  const line = (cls, from, to) => h("li", { class: `rail-line ${cls}`, "aria-hidden": "true", style: `grid-column:${from} / ${to}; --from:${from}; --to:${to}` });
  if (stop === -1) {
    rail.append(line("ok", 1, n + 1), line("ghost", 1, n + 1));
  } else {
    const k = stop + 1; // 1-based column of the failing step
    rail.append(line("ok", 1, k + 1));
    if (k < n) rail.append(line("skip", k + 1, n + 1));
    rail.append(h("li", { class: "rail-drop", "aria-hidden": "true", style: `grid-column:${k}; --at:${k}` }));
    if (!opts.crash) rail.append(line("err", k, n + 1));
  }

  steps.forEach((s, i) => {
    let cls = "station";
    let badge;
    if (GOOD.has(s.track)) badge = h("span", { class: "badge ok" }, labels[s.track] || s.track);
    else if (BAD.has(s.track)) { cls += " on-err"; badge = h("span", { class: "badge err" }, labels[s.track] || s.track); }
    else if (s.track === "panic") { cls += " panic"; badge = h("span", { class: "badge panic" }, "panic"); }
    else { cls += " skipped"; badge = h("span", { class: "badge skip" }, "skipped"); }
    const out = s.track === "skipped" ? null : s.output;
    rail.append(h("li", { class: cls, style: `--i:${i + 1}` },
      h("div", { class: "station-name" }, h("span", { class: "station-num" }, `${i + 1}`), s.name),
      h("div", { class: "badge-row" }, badge),
      out && h("div", { class: "station-out" }, out),
      s.note && h("div", { class: "station-note" }, s.note)));
  });
  return { el: rail, sig };
}

function httpChip(status, text) {
  const cls = status === 402 ? "s402" : `s${String(status)[0]}`;
  return h("span", { class: `http ${cls}` }, `${status}${text ? " " + text : ""}`);
}

// ---------- Go syntax highlight ----------

const KW = new Set("break case chan const continue default defer else fallthrough for func go goto if import interface map package range return select struct switch type var".split(" "));
const TYPES = new Set("string int int64 bool error any byte rune float64 nil true false".split(" "));
const PKGS = new Set("monadz tasks validations time context errors fmt json slices strings sync debug".split(" "));

function escapeHTML(s) {
  return s.replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
}

function highlightGo(code) {
  const re = /(\/\/[^\n]*)|("(?:[^"\\\n]|\\.)*"|`[^`]*`)|\b(\d+)\b|\b([A-Za-z_]\w*)\b(?=(\s*[(\[])?)/g;
  let out = "";
  let last = 0;
  let m;
  while ((m = re.exec(code))) {
    out += escapeHTML(code.slice(last, m.index));
    const [tok, com, str, num, ident, call] = m;
    let cls = null;
    if (com) cls = "com";
    else if (str) cls = "str";
    else if (num) cls = "num";
    else if (ident) {
      if (KW.has(ident)) cls = "kw";
      else if (TYPES.has(ident)) cls = "type";
      else if (PKGS.has(ident) && code[m.index + ident.length] === ".") cls = "type";
      else if (call && call.trim() === "(") cls = "fn";
    }
    out += cls ? `<span class="tok-${cls}">${escapeHTML(tok)}</span>` : escapeHTML(tok);
    last = m.index + tok.length;
  }
  return out + escapeHTML(code.slice(last));
}

// ---------- code card ----------

function codeCard(getSnippet) {
  let view = "monadz";
  const tabMonadz = h("button", { type: "button", class: "code-tab", role: "tab", "aria-selected": "true" });
  const tabPlain = h("button", { type: "button", class: "code-tab", role: "tab", "aria-selected": "false" });
  const pre = h("pre", { class: "code", tabindex: "0", "aria-label": "Go code" });
  const note = h("p", { class: "code-note" });
  const copyStatus = h("span", { class: "sr-only", role: "status" });
  const copyBtn = h("button", { type: "button", class: "btn btn-ghost btn-small" }, "Copy code");
  let current = { monadz: "", plain: "" };

  function render() {
    current = getSnippet();
    const lines = (s) => s.split("\n").length;
    clear(tabMonadz, "monadz", h("span", { class: "lines" }, `${lines(current.monadz)} lines`));
    clear(tabPlain, "plain Go", h("span", { class: "lines" }, `${lines(current.plain)} lines`));
    tabMonadz.setAttribute("aria-selected", String(view === "monadz"));
    tabPlain.setAttribute("aria-selected", String(view === "plain"));
    note.textContent = view === "monadz" ? current.note : "The same logic in plain Go, with no library.";
    pre.innerHTML = highlightGo(view === "monadz" ? current.monadz : current.plain);
  }
  tabMonadz.addEventListener("click", () => { view = "monadz"; render(); });
  tabPlain.addEventListener("click", () => { view = "plain"; render(); });
  copyBtn.addEventListener("click", async () => {
    const text = view === "monadz" ? current.monadz : current.plain;
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      const ta = h("textarea", { style: "position:fixed;opacity:0" });
      ta.value = text;
      document.body.append(ta);
      ta.select();
      try { document.execCommand("copy"); } catch { /* nothing more to try */ }
      ta.remove();
    }
    copyBtn.textContent = "Copied";
    copyStatus.textContent = "The code is copied to the clipboard.";
    setTimeout(() => { copyBtn.textContent = "Copy code"; copyStatus.textContent = ""; }, 1600);
  });

  const el = h("section", { class: "card code-card", "aria-label": "Code" },
    h("div", { class: "code-bar" },
      h("div", { class: "code-tabs", role: "tablist", "aria-label": "Code version" }, tabMonadz, tabPlain),
      h("div", { class: "code-actions" },
        copyBtn,
        h("a", { href: "https://go.dev/play/", target: "_blank", rel: "noopener" }, "Open the Go Playground"),
        copyStatus)),
    note, pre);
  render();
  return { el, render };
}

// ---------- scenarios ----------

const TASK_NAMES = ["usage", "invoices", "tax rates"];
const MODE_TEXT = {
  all: "Wait for all tasks. The first failure cancels the others.",
  any: "Return the first success. A failure does not stop the others.",
  race: "Return the first task to stop, success or failure. Cancel the others.",
  allSettled: "Wait for all tasks. Return every result. Cancel nothing.",
};

function taskSpecs(durations, fails) {
  return TASK_NAMES.map((name, i) => ({ name, durationMs: durations[i], fails: fails[i] }));
}


const PLAN_OPTS = [
  { value: "free", label: "free" },
  { value: "team", label: "team" },
  { value: "enterprise", label: "enterprise" },
];
const FEATURE_OPTS = [
  { value: "sso", label: "sso" },
  { value: "audit_log", label: "audit_log" },
  { value: "api_access", label: "api_access" },
];

const signupFields = (withSwitches) => [
  { type: "text", key: "email", label: "Email" },
  { type: "text", key: "password", label: "Password", hint: "At least 8 characters, with a digit. The playground shows it in clear text." },
  { type: "text", key: "company", label: "Company" },
  ...(withSwitches ? [
    { type: "switch", key: "emailTaken", label: "Email already taken", small: "Step 2 fails" },
    { type: "switch", key: "dbDown", label: "Database down", small: "Step 4 fails" },
  ] : []),
];

const SCENARIOS = [
  {
    id: "option", name: "Option", sub: "can be missing", type: "Option[T]",
    title: "The value can be missing",
    explain: "An Option holds a value (Some) or nothing (None). When a step finds nothing, the later steps do not run, and you give one default at the end.",
    story: "The story: a visitor opens the login page of a tenant. The page finds the tenant from the host name and shows the tenant's colors.",
    defaults: { host: "www.Acme.app.example:443" },
    presets: [
      { label: "Happy path", values: { host: "www.Acme.app.example:443" } },
      { label: "Unknown domain (step 2)", values: { host: "contoso.app.example" } },
      { label: "Suspended tenant (step 3)", values: { host: "globex.app.example" } },
      { label: "No branding (step 4)", values: { host: "initech.app.example" } },
    ],
    fields: (s) => [
      { type: "text", key: "host", label: "Host name", hint: "Type any host. The page ignores case, \"www.\", and the port." },
      { type: "node", node: directoryNode(s) },
    ],
    run: (s) => callSync("option", s),
    render(d, ctx) {
      const rail = renderRail(d.steps, { some: "Some", none: "None" }, ctx.sig);
      ctx.sig = rail.sig;
      ctx.directory(d.directory, d.steps[0]);
      const b = d.branding;
      return [
        h("div", { class: "result-head" }, h("h3", { class: "card-title" }, "Pipeline"), legend("Some", "None")),
        rail.el,
        h("div", { class: `answer ${d.usedDefault ? "" : "ok"}` },
          h("div", { class: "answer-title" }, "ValueOr(DefaultBranding) gives the login page:"),
          h("div", { class: "login-preview" },
            h("span", { class: "login-swatch", style: `background:${b.color}` }),
            h("div", {},
              h("div", { class: "login-title" }, b.title),
              h("div", { class: "login-sub mono" }, b.color))),
          h("p", {}, d.usedDefault
            ? "The chain ended in None, so ValueOr used the default branding. The code checks for \"missing\" only once, at the end."
            : "The chain ended in Some, so the page uses the tenant's branding.")),
      ];
    },
  },
  {
    id: "result", name: "Result", sub: "can fail", type: "Result[T]",
    title: "The step can fail",
    explain: "A Result holds a value (Ok) or an error (Err). When a step fails, the value drops to the Err track, and the later steps do not run.",
    story: "The story: a new customer signs up. The service checks the form, checks that the email is free, hashes the password, and inserts the account.",
    defaults: { email: "ada@acme.example", password: "correct1horse", company: "Acme", emailTaken: false, dbDown: false },
    presets: [
      { label: "Happy path", values: { email: "ada@acme.example", password: "correct1horse", company: "Acme", emailTaken: false, dbDown: false } },
      { label: "Bad email (step 1)", values: { email: "ada-at-acme", password: "correct1horse", company: "Acme", emailTaken: false, dbDown: false } },
      { label: "Email taken (step 2)", values: { email: "ada@acme.example", password: "correct1horse", company: "Acme", emailTaken: true, dbDown: false } },
      { label: "Database down (step 4)", values: { email: "ada@acme.example", password: "correct1horse", company: "Acme", emailTaken: false, dbDown: true } },
    ],
    fields: () => signupFields(true),
    run: (s) => callSync("result", s),
    render(d, ctx) {
      const rail = renderRail(d.steps, { ok: "Ok", err: "Err" }, ctx.sig);
      ctx.sig = rail.sig;
      const ok = d.status < 300;
      return [
        h("div", { class: "result-head" }, h("h3", { class: "card-title" }, "Railway"), legend("Ok track", "Err track")),
        rail.el,
        h("div", { class: `answer ${ok ? "ok" : "err"}` },
          h("div", { class: "answer-title" }, "Match gives the HTTP response:", httpChip(d.status, d.statusText)),
          h("pre", {}, d.body),
          h("p", {}, ok
            ? "Every step ran on the Ok track."
            : "Match runs the error branch once, at the end. Each step does not need its own \"if err != nil\".")),
      ];
    },
  },
  {
    id: "validation", name: "Validation", sub: "all the errors", type: "Validation[T]",
    title: "Report all the errors at once",
    explain: "A Result stops at the first error, so the user fixes one field at a time. A Validation runs every check and returns all the errors together.",
    story: "The story: the same signup form. Clear some fields, and compare what each type reports to the user.",
    defaults: { email: "ada", password: "abc", company: "" },
    presets: [
      { label: "Three bad fields", values: { email: "ada", password: "abc", company: "" } },
      { label: "One bad field", values: { email: "ada@acme.example", password: "correcthorse", company: "Acme" } },
      { label: "All valid", values: { email: "ada@acme.example", password: "correct1horse", company: "Acme" } },
    ],
    fields: () => signupFields(false),
    run: (s) => callSync("validation", s),
    render(d, ctx) {
      const byField = {};
      for (const e of d.errors) (byField[e.field] ||= []).push(e.message);
      ctx.form.setErrors(byField);
      const list = (errs) => h("ul", { class: "err-list" },
        errs.length ? errs.map((e) => h("li", {}, h("code", {}, e.field), e.message)) : h("li", { class: "none" }, "No errors"));
      const n = d.errors.length;
      return [
        h("div", { class: "result-head" }, h("h3", { class: "card-title" }, "What the user sees"), httpChip(d.status, d.status === 200 ? "OK" : "Unprocessable Entity")),
        h("div", { class: "compare" },
          h("div", { class: `compare-col ${n > 1 ? "winner" : ""}` },
            h("h3", {}, "Validation", h("span", { class: `count ${n ? "" : "zero"}` }, `${n} error${n === 1 ? "" : "s"}`)),
            h("p", { class: "compare-sub" }, "validations.Map3 + Field"),
            list(d.errors)),
          h("div", { class: "compare-col" },
            h("h3", {}, "Result", h("span", { class: `count ${d.resultErrors.length ? "" : "zero"}` }, `${d.resultErrors.length} error${d.resultErrors.length === 1 ? "" : "s"}`)),
            h("p", { class: "compare-sub" }, "a chain of Ensure calls"),
            list(d.resultErrors))),
        h("div", { class: `answer ${n ? (n > 1 ? "warn" : "err") : "ok"}` },
          h("div", { class: "answer-title" }, n > 1
            ? `The Result user must submit the form ${n} times to find all ${n} errors.`
            : n === 1 ? "With one error, both types report the same thing." : "Both types accept the form."),
          h("p", { class: "mono" }, d.summary)),
      ];
    },
  },
  {
    id: "either", name: "Either", sub: "two valid answers", type: "Either[L, R]",
    title: "Two valid outcomes",
    explain: "An Either holds one of two valid answers: Left or Right. A paywall is not an error: the request worked, and the answer is \"upgrade first\".",
    story: "The story: a tenant clicks a feature. The service checks the plan and the seat count, and answers with access or with an upgrade offer.",
    defaults: { plan: "team", feature: "sso", seats: 12 },
    presets: [
      { label: "Enterprise + SSO (200)", values: { plan: "enterprise", feature: "sso", seats: 40 } },
      { label: "Team + SSO (402)", values: { plan: "team", feature: "sso", seats: 12 } },
      { label: "Free, too many seats (402)", values: { plan: "free", feature: "api_access", seats: 10 } },
    ],
    fields: () => [
      { type: "seg", key: "plan", label: "Plan", options: PLAN_OPTS },
      { type: "seg", key: "feature", label: "Feature", options: FEATURE_OPTS },
      { type: "range", key: "seats", label: "Seats used", min: 1, max: 60, hint: "free: 3 seats, team: 25, enterprise: 500." },
    ],
    run: (s) => callSync("either", s),
    render(d, ctx) {
      const rail = renderRail(d.steps, { right: "Right", left: "Left" }, ctx.sig);
      ctx.sig = rail.sig;
      const p = d.paywall;
      const g = d.grant;
      return [
        h("div", { class: "result-head" }, h("h3", { class: "card-title" }, "Two tracks, both valid"), legend("Right (Grant)", "Left (Paywall)")),
        rail.el,
        h("div", { class: "either-sides" },
          h("div", { class: `side left ${p ? "active" : ""}` },
            h("h3", {}, "Left: Paywall ", p && httpChip(402, "Payment Required")),
            p ? [
              h("p", {}, p.reason),
              p.priceUsd > 0
                ? h("div", { class: "price" }, `Upgrade to ${p.upgradeTo}: $${p.priceUsd}`, h("small", {}, " / month"))
                : h("div", { class: "price" }, `Upgrade: ${p.upgradeTo}`),
            ] : h("p", {}, "Not this time.")),
          h("div", { class: `side right ${g ? "active" : ""}` },
            h("h3", {}, "Right: Grant ", g && httpChip(200, "OK")),
            g ? h("p", {}, `${g.feature} is on for this tenant (${g.plan} plan).`) : h("p", {}, "Not this time."))),
        h("div", { class: "answer" }, h("div", { class: "answer-title" }, "Response body"), h("pre", {}, d.body)),
        h("p", { class: "why" }, h("strong", {}, "Why not an error? "),
          "An error means that something broke, and it sets off logs and alerts. A paywall is a normal answer with data: the plan and the price. Either keeps both answers typed, and Match makes the caller handle both."),
      ];
    },
  },
  {
    id: "patch", name: "Patch", sub: "null vs not sent", type: "Patch[T]",
    title: "Null is not the same as \"not sent\"",
    explain: "In a JSON PATCH body, a field can be missing, null, or set to a value. An Option has only two states, so it cannot tell \"keep the logo\" from \"clear the logo\"; a Patch has three.",
    story: "The story: an admin sends PATCH /tenant to change the tenant settings. Edit the JSON body below.",
    defaults: { body: '{"name": "Acme Corp", "logo": null}' },
    presets: [
      { label: "Rename + clear logo", values: { body: '{"name": "Acme Corp", "logo": null}' } },
      { label: "Only rename", values: { body: '{"name": "Acme Corp"}' } },
      { label: "Clear the name (422)", values: { body: '{"name": null}' } },
      { label: "Bad JSON (400)", values: { body: '{"name": "Acme",' } },
    ],
    fields: () => [
      { type: "textarea", key: "body", label: "PATCH /tenant body (JSON)", rows: 6, hint: "Fields: name, logo, support_email. Use null to clear a field. Leave a field out to keep it." },
    ],
    run: (s) => callSync("patch", s),
    render(d, ctx) {
      ctx.form.setErrors(d.status === 400 ? { body: [d.error] } : {});
      if (d.status === 400) {
        return [h("div", { class: "answer err", role: "alert" },
          h("div", { class: "answer-title" }, httpChip(400, "Bad Request"), "The body is not valid for UpdateTenant"),
          h("pre", {}, d.error))];
      }
      const wrong = d.fields.filter((f) => f.differs);
      const rows = d.fields.map((f) => h("tr", {},
        h("th", { scope: "row", class: "mono" }, f.field),
        h("td", { class: "mono" }, f.sent),
        h("td", {}, h("span", { class: `state ${f.state}` }, f.state)),
        h("td", { class: "mono" }, f.before),
        h("td", { class: `mono ${f.after !== f.before ? "changed" : ""}` }, f.after),
        h("td", {}, h("span", { class: `state ${f.optionState}` }, f.optionState)),
        h("td", { class: `mono ${f.differs ? "wrong" : ""}` }, f.optionAfter, f.differs ? " ✗" : "")));
      return [
        h("div", { class: "result-head" }, h("h3", { class: "card-title" }, "The stored tenant, before and after"), httpChip(d.status, d.status === 200 ? "OK" : "Unprocessable Entity")),
        d.error && h("div", { class: "answer err", role: "alert" }, h("div", { class: "answer-title" }, `422: ${d.error}`), h("p", {}, "The handler checks Name.IsNull() and stops. Nothing changes.")),
        h("div", { class: "patch-table-wrap" }, h("table", { class: "patch-table" },
          h("thead", {},
            h("tr", {}, h("td", { colspan: 2 }), h("th", { colspan: 3, class: "th-group", scope: "colgroup" }, "with Patch[string]"), h("th", { colspan: 2, class: "th-group", scope: "colgroup" }, "with Option[string]")),
            h("tr", {}, ["field", "sent", "state", "before", "after", "state", "after"].map((t) => h("th", { scope: "col" }, t)))),
          h("tbody", {}, rows))),
        h("div", { class: `answer ${wrong.length ? "err" : "ok"}` },
          h("div", { class: "answer-title" }, wrong.length
            ? `The Option version gets ${wrong.length} field${wrong.length > 1 ? "s" : ""} wrong: ${wrong.map((f) => f.field).join(", ")}.`
            : "Here both versions give the same record."),
          h("p", {}, wrong.length
            ? "A missing field and null both decode to None. The Option code must treat None as \"keep\", or a missing field clears the value. So null cannot clear anything."
            : "Send a field as null (for example \"logo\": null) to see where the Option version goes wrong.")),
      ];
    },
  },
  {
    id: "do", name: "Do", sub: "Try and panics", type: "Do / DoRecover",
    title: "Do and DoRecover: Go's ? operator",
    explain: "Inside Do, Try() gives the value of a Result, or stops the block at the first Err. Do catches only this stop, so a real bug still crashes the program; DoRecover also turns a bug into an Err.",
    story: "The story: the billing job builds the monthly invoice for a tenant in one block of straight-line code.",
    defaults: { failStep2: false, bug: false, mode: "do" },
    presets: [
      { label: "Happy path", values: { failStep2: false, bug: false, mode: "do" } },
      { label: "Fails at step 2", values: { failStep2: true, bug: false, mode: "do" } },
      { label: "Bug in Do (crash)", values: { failStep2: false, bug: true, mode: "do" } },
      { label: "Bug in DoRecover", values: { failStep2: false, bug: true, mode: "dorecover" } },
    ],
    fields: () => [
      { type: "seg", key: "mode", label: "Run the block with", options: [{ value: "do", label: "monadz.Do" }, { value: "dorecover", label: "monadz.DoRecover" }] },
      { type: "switch", key: "failStep2", label: "Make step 2 fail", small: "The plan is not in the price list" },
      { type: "switch", key: "bug", label: "Add a bug", small: "Write to a nil map at step 3" },
    ],
    run: (s) => callSync("do", s),
    render(d, ctx) {
      const rail = renderRail(d.steps, { ok: "ok", err: "Err" }, ctx.sig, { crash: d.outcome === "crash" });
      ctx.sig = rail.sig;
      let answer;
      switch (d.outcome) {
        case "ok":
          answer = h("div", { class: "answer ok" }, h("div", { class: "answer-title" }, h("span", { class: "badge ok" }, "Ok"), d.value),
            h("p", {}, "Every Try got a value, so the block returned normally."));
          break;
        case "err":
          answer = h("div", { class: "answer err" }, h("div", { class: "answer-title" }, h("span", { class: "badge err" }, "Err"), d.error),
            h("p", {}, "Try got an Err at step 2 and stopped the block. Steps 3 and 4 did not run, so the bug at step 3 did not run either."));
          break;
        case "crash":
          answer = h("div", { class: "answer panic", role: "alert" },
            h("div", { class: "answer-title" }, "panic: the program would crash here"),
            h("pre", {}, d.error),
            h("p", {}, "Do recovers only the stop signal from Try. This panic is a bug, so Do lets it pass. In a real program, the process stops and prints the stack trace. The playground caught the panic at its own boundary only to show it to you."));
          break;
        default:
          answer = h("div", { class: "answer warn" },
            h("div", { class: "answer-title" }, h("span", { class: "badge warn" }, "Err(*PanicError)"), d.error),
            h("p", {}, "DoRecover turned the panic into an Err. An HTTP handler can return 500, and the server keeps running. The error keeps the stack trace for the logs."),
            h("details", {}, h("summary", {}, "Stack trace (first lines)"), h("pre", {}, d.stack)));
      }
      return [
        h("div", { class: "result-head" }, h("h3", { class: "card-title" }, `Inside monadz.${d.mode === "do" ? "Do" : "DoRecover"}`), legend("ran", "stopped")),
        rail.el,
        answer,
        h("p", { class: "why" }, h("strong", {}, "Why does Do let a bug crash? "),
          "A bug is not a business failure. If Do caught every panic, a nil map write would look like a normal Err, and nobody would fix it. Use DoRecover only at a boundary, for example an HTTP handler or a queue consumer."),
      ];
    },
  },
  tasksScenario(),
  retryScenario(),
];

// directoryNode makes the tenant table for the Option inputs. The table
// fills in after the first run, from the data that Go returns.
function directoryNode() {
  const body = h("tbody", {});
  const node = h("details", { class: "more", open: true },
    h("summary", {}, "The tenant directory"),
    h("table", { class: "directory" },
      h("thead", {}, h("tr", {}, h("th", { scope: "col" }, "host"), h("th", { scope: "col" }, "status"), h("th", { scope: "col" }, "branding"))),
      body));
  node._fill = (rows, first) => {
    const host = first && first.track === "some" ? first.output.replace(/^Some\("(.*)"\)$/, "$1") : "";
    clear(body, rows.map((r) => h("tr", { class: r.host === host ? "match" : null },
      h("td", { class: "mono" }, r.host),
      h("td", {}, r.status),
      h("td", {}, r.branding === "none" ? "none" : [h("span", { class: "dot", style: `background:${r.branding}` }), r.branding]))));
  };
  return node;
}

// ---------- Tasks ----------

function tasksScenario() {
  return {
    id: "tasks", name: "Tasks", sub: "run at the same time", type: "Task[T] + package tasks",
    title: "Work that runs at the same time",
    explain: "A Task is work that runs later and takes a context. The functions in package tasks run many tasks at the same time and cancel the tasks that are no longer necessary.",
    story: "The story: the billing page loads usage, invoices, and tax rates from three services. Set each task, choose a function, and press Run. The tasks run for real, with real timers.",
    async: true,
    defaults: { mode: "all", tasks: taskSpecs([600, 1200, 1800], [false, true, false]) },
    presets: [
      { label: "All: one fails, the rest cancel", values: { mode: "all", tasks: taskSpecs([600, 1000, 1800], [false, true, false]) } },
      { label: "All: every task succeeds", values: { mode: "all", tasks: taskSpecs([600, 1200, 900], [false, false, false]) } },
      { label: "Race: the fastest wins", values: { mode: "race", tasks: taskSpecs([900, 400, 1500], [false, false, false]) } },
      { label: "Any: skip a fast failure", values: { mode: "any", tasks: taskSpecs([300, 800, 1400], [true, false, false]) } },
      { label: "AllSettled: keep every result", values: { mode: "allSettled", tasks: taskSpecs([500, 1000, 1500], [false, true, false]) } },
    ],
    fields: () => [
      { type: "modes", key: "mode", label: "Function", options: [
        { value: "all", code: "tasks.All", desc: MODE_TEXT.all },
        { value: "any", code: "tasks.Any", desc: MODE_TEXT.any },
        { value: "race", code: "tasks.Race", desc: MODE_TEXT.race },
        { value: "allSettled", code: "tasks.AllSettled", desc: MODE_TEXT.allSettled },
      ] },
      ...TASK_NAMES.map((name, i) => ({ type: "group", legend: `Task ${i + 1}: ${name}`, fields: [
        { type: "range", key: `tasks.${i}.durationMs`, label: "Duration", min: 100, max: 2000, step: 50, format: (v) => ms(v) },
        { type: "seg", key: `tasks.${i}.fails`, label: `Outcome of ${name}`, hideLegend: true, cls: "ok-err", options: [{ value: "false", label: "succeeds" }, { value: "true", label: "fails" }] },
      ] })),
    ],
    runAsync: (s, onEvent) => callAsync("tasks", s, onEvent),
    initial: () => h("div", { class: "empty" }, "Press Run to start the three tasks."),
    live: liveTasks,
    render: renderTasks,
  };
}

function laneState(events, name) {
  const st = { start: null, end: null, kind: null };
  for (const e of events) {
    if (e.task !== name) continue;
    if (e.kind === "start") st.start = e.atMs;
    else { st.end = e.atMs; st.kind = e.kind; }
  }
  return st;
}

function axis(scale) {
  // round tick values: 100, 200, 250, 500, or 1000 ms apart
  const step = [100, 200, 250, 500, 1000, 2000].find((st) => scale / st <= 5) || 2000;
  const ticks = [];
  for (let v = 0; v <= scale; v += step) ticks.push(h("span", { style: `left:${(v / scale) * 100}%` }, ms(v)));
  return h("div", { class: "axis", "aria-hidden": "true" }, h("span"), h("div", { class: "axis-scale" }, ticks));
}

function drawTimeline(specs, events, now, scale, running) {
  const pct = (v) => `${Math.max(0, Math.min(100, (v / scale) * 100))}%`;
  const lanes = specs.map((t) => {
    const st = laneState(events, t.name);
    const track = h("div", { class: "lane-track" },
      h("div", { class: "lane-plan", style: `width:${pct(t.durationMs)}`, title: `planned: ${ms(t.durationMs)}` }));
    let label = "waiting";
    if (st.start !== null) {
      const end = st.end ?? now;
      const kind = st.kind ?? "running";
      const text = { running: "running", ok: `ok · ${ms(end)}`, fail: `failed · ${ms(end)}`, canceled: `canceled at ${ms(end)}` }[kind];
      label = text;
      track.append(h("div", { class: `bar ${kind}`, style: `left:${pct(st.start)}; width:calc(${pct(end)} - ${pct(st.start)})` }, h("span", {}, text)));
    }
    return h("div", { class: "lane", role: "listitem", "aria-label": `${t.name}: ${label}` },
      h("div", { class: "lane-name" }, t.name, h("small", {}, `${ms(t.durationMs)} · ${t.fails ? "fails" : "ok"}`)),
      track);
  });
  const tl = h("div", { class: "timeline" }, h("div", { role: "list", "aria-label": "Task timeline" }, lanes), axis(scale));
  if (running) {
    // the now line, over the lane tracks only
    const wrap = h("div", { style: "position:absolute; inset:0 0 16px 0; pointer-events:none", "aria-hidden": "true" });
    const inner = h("div", { style: "position:absolute; top:0; bottom:0; right:0; left:calc(var(--lane-label) + var(--lane-gap))" },
      h("div", { class: "now-line", style: `left:${pct(now)}` }));
    wrap.append(inner);
    tl.append(wrap);
  }
  return tl;
}

function liveTasks(state, events, now) {
  const scale = Math.max(...state.tasks.map((t) => t.durationMs)) * 1.06;
  return [
    h("div", { class: "result-head" }, h("h3", { class: "card-title" }, `Running tasks.${modeName(state.mode)}…`), timelineLegend()),
    drawTimeline(state.tasks, events, now, scale, true),
  ];
}

function modeName(m) {
  return { all: "All", any: "Any", race: "Race", allSettled: "AllSettled" }[m] || "All";
}

function timelineLegend() {
  return h("div", { class: "legend", "aria-hidden": "true" },
    h("span", {}, h("i", { class: "l-ok" }), "ok"),
    h("span", {}, h("i", { class: "l-err" }), "failed"),
    h("span", {}, h("i", { class: "l-skip" }), "canceled / planned"));
}

function renderTasks(d) {
  const scale = Math.max(d.totalMs, ...d.tasks.map((t) => t.durationMs)) * 1.06;
  const canceled = d.tasks.filter((t) => laneState(d.events, t.name).kind === "canceled").map((t) => t.name);
  const failed = d.tasks.filter((t) => laneState(d.events, t.name).kind === "fail").map((t) => t.name);
  const fn = `tasks.${modeName(d.mode)}`;
  let story;
  switch (d.mode) {
    case "all":
      story = failed.length
        ? `${failed[0]} failed, so ${fn} canceled the tasks that still ran${canceled.length ? `: ${canceled.join(", ")}` : ""}. It waited for them to stop, then returned the error.`
        : `Every task succeeded. ${fn} returned all the values, in input order.`;
      break;
    case "any":
      story = d.ok
        ? `${fn} returned the first success.${failed.length ? ` The failure of ${failed.join(", ")} did not stop the others.` : ""}${canceled.length ? ` Then it canceled ${canceled.join(", ")}.` : ""}`
        : `Every task failed, so ${fn} returned all the errors, joined.`;
      break;
    case "race":
      story = `${fn} returned the first task to stop${d.ok ? "" : ", and that task failed"}.${canceled.length ? ` It canceled ${canceled.join(", ")}.` : ""}`;
      break;
    default:
      story = `${fn} waited for every task and canceled nothing. Each task has its own Result.`;
  }
  const isOk = d.ok && !(d.mode === "allSettled" && failed.length);
  return [
    h("div", { class: "result-head" }, h("h3", { class: "card-title" }, `${fn} · ${ms(d.totalMs)} in total`), timelineLegend()),
    drawTimeline(d.tasks, d.events, d.totalMs, scale, false),
    h("div", { class: `answer ${d.mode === "allSettled" ? (failed.length ? "warn" : "ok") : isOk ? "ok" : "err"}` },
      h("div", { class: "answer-title" },
        d.mode === "allSettled" ? h("span", { class: "badge ok" }, "Ok") : h("span", { class: `badge ${d.ok ? "ok" : "err"}` }, d.ok ? "Ok" : "Err"),
        h("span", { class: "mono" }, d.ok ? d.value : d.error)),
      d.settled && h("ul", { class: "settled" }, d.settled.map((s) => h("li", {}, `${s.task}: ${s.ok ? `Ok(${JSON.stringify(s.value)})` : `Err(${s.error})`}`))),
      h("p", {}, story)),
  ];
}

// ---------- Retry ----------

function backoffMs(base, n) {
  return Math.min(base * 2 ** (n - 1), 2000);
}

function retryScenario() {
  return {
    id: "retry", name: "Retry", sub: "try again, wait more", type: "Task.Retry",
    title: "Retry with exponential backoff",
    explain: "Retry runs a failed Task again, as the RetryPolicy allows. ExponentialBackoff doubles the wait after each failure, up to a limit.",
    story: "The story: the billing job charges a card. The payment gateway times out a few times, then answers. Each attempt takes 150 ms.",
    async: true,
    defaults: { failuresBeforeSuccess: 2, maxAttempts: 4, baseMs: 200 },
    presets: [
      { label: "Succeeds on attempt 3", values: { failuresBeforeSuccess: 2, maxAttempts: 4, baseMs: 200 } },
      { label: "Gives up", values: { failuresBeforeSuccess: 5, maxAttempts: 3, baseMs: 200 } },
      { label: "Succeeds at once", values: { failuresBeforeSuccess: 0, maxAttempts: 4, baseMs: 200 } },
    ],
    fields: (s) => {
      const plan = h("span", { class: "field-hint", "aria-live": "polite" });
      const update = () => {
        const waits = [];
        for (let n = 1; n < s.maxAttempts; n++) waits.push(ms(backoffMs(s.baseMs, n)));
        plan.textContent = waits.length ? `Waits between the attempts: ${waits.join(", ")}.` : "One attempt: Retry does not wait.";
      };
      plan._update = update;
      update();
      return [
        { type: "range", key: "failuresBeforeSuccess", label: "The charge fails this many times, then succeeds", min: 0, max: 6, format: (v) => `${v}×` },
        { type: "range", key: "maxAttempts", label: "MaxAttempts", min: 1, max: 6, format: (v) => `${v}` },
        { type: "range", key: "baseMs", label: "Base backoff", min: 50, max: 800, step: 50, format: (v) => ms(v) },
        { type: "node", node: plan },
      ];
    },
    runAsync: (s, onEvent) => callAsync("retry", s, onEvent),
    initial: () => h("div", { class: "empty" }, "Press Run to charge the card."),
    live: (s, events, now) => [
      h("div", { class: "result-head" }, h("h3", { class: "card-title" }, "Charging…"), retryLegend()),
      drawRetry(s, events, now, retryScale(s), true),
    ],
    render: (d, ctx) => {
      const s = ctx.lastInput;
      const scale = Math.max(retryScale(s), d.totalMs * 1.04);
      const rows = d.attempts.map((a, i) => h("tr", {},
        h("td", {}, `#${a.n}`),
        h("td", { class: "mono" }, ms(a.startMs)),
        h("td", {}, a.ok ? h("span", { class: "badge ok" }, "ok") : h("span", { class: "badge err", title: a.error || "" }, "timeout")),
        h("td", { class: "mono" }, i < d.attempts.length - 1 ? ms(d.attempts[i + 1].startMs - a.endMs) : "—")));
      return [
        h("div", { class: "result-head" }, h("h3", { class: "card-title" }, `${d.attempts.length} attempt${d.attempts.length > 1 ? "s" : ""} · ${ms(d.totalMs)}`), retryLegend()),
        drawRetry(s, d.events, d.totalMs, scale, false),
        h("div", { class: "patch-table-wrap" }, h("table", { class: "directory" },
          h("thead", {}, h("tr", {}, ["attempt", "started", "result", "wait after"].map((t) => h("th", { scope: "col" }, t)))),
          h("tbody", {}, rows))),
        h("div", { class: `answer ${d.ok ? "ok" : "err"}` },
          h("div", { class: "answer-title" }, h("span", { class: `badge ${d.ok ? "ok" : "err"}` }, d.ok ? "Ok" : "Err"), h("span", { class: "mono" }, d.ok ? d.value : d.error)),
          h("p", {}, d.ok
            ? (d.attempts.length > 1 ? `Retry hid ${d.attempts.length - 1} failure${d.attempts.length > 2 ? "s" : ""} from the caller. The waits doubled: ${d.planMs.slice(0, d.attempts.length - 1).map(ms).join(", ")}.` : "The first attempt succeeded, so Retry did not wait.")
            : `Retry stopped after MaxAttempts (${s.maxAttempts}). The error tells how many attempts ran, and errors.Is still finds the gateway error.`)),
      ];
    },
  };
}

function retryLegend() {
  return h("div", { class: "legend", "aria-hidden": "true" },
    h("span", {}, h("i", { class: "l-ok" }), "attempt ok"),
    h("span", {}, h("i", { class: "l-err" }), "attempt failed"),
    h("span", {}, h("i", { class: "l-skip" }), "wait"));
}

function retryScale(s) {
  const attempts = Math.min(s.failuresBeforeSuccess + 1, s.maxAttempts);
  let total = attempts * 150;
  for (let n = 1; n < attempts; n++) total += backoffMs(s.baseMs, n);
  return total * 1.08;
}

function drawRetry(s, events, now, scale, running) {
  const pct = (v) => `${Math.max(0, Math.min(100, (v / scale) * 100))}%`;
  const attempts = [];
  for (const e of events) {
    const n = Number(e.task.replace("attempt ", ""));
    const a = (attempts[n - 1] ||= { n, start: null, end: null, kind: null });
    if (e.kind === "start") a.start = e.atMs;
    else { a.end = e.atMs; a.kind = e.kind; }
  }
  const track = h("div", { class: "lane-track", style: "height:44px" });
  attempts.forEach((a, i) => {
    if (!a || a.start === null) return;
    const end = a.end ?? now;
    const kind = a.kind ?? "running";
    track.append(h("div", { class: `bar ${kind}`, style: `left:${pct(a.start)}; width:calc(${pct(end)} - ${pct(a.start)})`, title: `attempt ${a.n}: ${kind}` }, h("span", {}, `#${a.n}`)));
    // the wait after a failed attempt
    if (a.kind === "fail" && a.n < s.maxAttempts) {
      const next = attempts[i + 1];
      const waitEnd = next && next.start !== null ? next.start : running ? now : null;
      if (waitEnd !== null && waitEnd > a.end) {
        track.append(h("div", { class: "bar wait", style: `left:${pct(a.end)}; width:calc(${pct(waitEnd)} - ${pct(a.end)})`, title: `wait ${ms(waitEnd - a.end)}` }, h("span", {}, "wait")));
      }
    }
  });
  const done = attempts.filter((a) => a && a.kind);
  const label = running ? "running" : done.length ? `${done.length} attempts` : "";
  return h("div", { class: "timeline" },
    h("div", { role: "list", "aria-label": "Retry timeline" },
      h("div", { class: "lane", role: "listitem", "aria-label": `charge card: ${label}` },
        h("div", { class: "lane-name" }, "charge card", h("small", {}, `max ${s.maxAttempts} attempts`)),
        track)),
    axis(scale));
}

// ---------- panels ----------

const tablist = document.getElementById("tablist");
const panelsEl = document.getElementById("panels");
const panels = new Map();

function buildPanel(sc, index) {
  const state = structuredClone(sc.defaults);
  const ctx = { sig: "", form: null, lastInput: null, directory: () => {} };
  const resultBody = h("div", { "aria-live": sc.async ? "polite" : "off" });
  const resultCard = h("section", { class: "card result-card", "aria-label": "Result" }, resultBody);
  const code = codeCard(() => snippets[sc.id](state));

  let presetButtons = [];
  let runBtn = null;
  let running = false;
  let ran = false;
  let staleNote = null;

  const compute = () => {
    if (!ready) return;
    try {
      const env = sc.run(state);
      if (!env.ok) { clear(resultBody, bridgeError(env.error)); return; }
      clear(resultBody, sc.render(env.data, ctx));
    } catch (err) {
      clear(resultBody, bridgeError(String(err && err.stack || err)));
    }
  };
  const debounced = debounce(compute, 150);

  async function run() {
    if (!ready || running) return;
    running = true;
    ran = true;
    runBtn.disabled = true;
    runBtn.textContent = "Running…";
    if (staleNote) staleNote.textContent = "";
    const input = structuredClone(state);
    ctx.lastInput = input;
    const events = [];
    const t0 = performance.now();
    let frame = 0;
    const tick = () => {
      clear(resultBody, sc.live(input, events, performance.now() - t0));
      frame = requestAnimationFrame(tick);
    };
    tick();
    try {
      const env = await sc.runAsync(input, (e) => events.push(e));
      cancelAnimationFrame(frame);
      if (!env.ok) clear(resultBody, bridgeError(env.error));
      else clear(resultBody, sc.render(env.data, ctx));
    } catch (err) {
      cancelAnimationFrame(frame);
      clear(resultBody, bridgeError(String(err && err.stack || err)));
    } finally {
      running = false;
      runBtn.disabled = false;
      runBtn.textContent = "Run again";
    }
  }

  const onChange = () => {
    for (const b of presetButtons) b.setAttribute("aria-pressed", "false");
    code.render();
    if (sc.async) {
      if (ran && staleNote) staleNote.textContent = "The inputs changed. Press Run to see the new result.";
      if (fieldsExtra.update) fieldsExtra.update();
    } else {
      debounced();
    }
  };

  const fieldsExtra = {};
  const fieldDefs = sc.fields(state);
  for (const f of fieldDefs) {
    if (f.type === "node" && f.node._update) fieldsExtra.update = f.node._update;
    if (f.type === "node" && f.node._fill) ctx.directory = f.node._fill;
  }
  const form = buildForm(fieldDefs, state, onChange);
  ctx.form = form;

  function applyPreset(p, btn) {
    const fresh = structuredClone({ ...sc.defaults, ...p.values });
    for (const k of Object.keys(state)) delete state[k];
    Object.assign(state, fresh);
    form.sync();
    onChange();
    for (const b of presetButtons) b.setAttribute("aria-pressed", String(b === btn));
    if (sc.async) run();
  }

  presetButtons = sc.presets.map((p) => {
    const b = h("button", { type: "button", class: "preset", "aria-pressed": "false" }, p.label);
    b.addEventListener("click", () => applyPreset(p, b));
    return b;
  });
  if (presetButtons[0] && !sc.async) presetButtons[0].setAttribute("aria-pressed", "true");

  const inputsCard = h("section", { class: "card", "aria-label": "Inputs" }, h("h3", { class: "card-title" }, "Inputs"), form.el);
  if (sc.async) {
    runBtn = h("button", { type: "button", class: "btn btn-primary btn-block" }, "Run");
    runBtn.addEventListener("click", run);
    staleNote = h("span", { class: "run-hint", role: "status" });
    inputsCard.append(h("div", { class: "run-row" }, runBtn, staleNote));
    clear(resultBody, sc.initial());
  } else {
    clear(resultBody, h("div", { class: "empty" }, "Waiting for the Go library…"));
  }

  const prev = SCENARIOS[index - 1];
  const next = SCENARIOS[index + 1];
  const panel = h("section", { class: "panel", id: `panel-${sc.id}`, role: "tabpanel", "aria-labelledby": `tab-${sc.id}`, hidden: true },
    h("div", { class: "panel-head" },
      h("p", { class: "eyebrow" }, `${index + 1} of ${SCENARIOS.length} · ${sc.type}`),
      h("h2", {}, sc.title),
      h("p", { class: "explain" }, sc.explain),
      h("p", { class: "story" }, sc.story)),
    h("div", { class: "presets", role: "group", "aria-label": "Presets" }, h("span", { class: "presets-label" }, "Try:"), presetButtons),
    h("div", { class: "grid" }, inputsCard, resultCard),
    code.el,
    h("nav", { class: "panel-nav", "aria-label": "Scenario navigation" },
      prev ? h("a", { class: "btn btn-ghost", href: `#${prev.id}` }, `← ${prev.name}`) : h("span"),
      next ? h("a", { class: "btn btn-ghost", href: `#${next.id}` }, `Next: ${next.name} →`) : h("span")));

  return { panel, compute, sc, computed: false };
}

function buildTabs() {
  SCENARIOS.forEach((sc, i) => {
    const tab = h("button", {
      type: "button", class: "tab", role: "tab", id: `tab-${sc.id}`,
      "aria-controls": `panel-${sc.id}`, "aria-selected": "false", tabindex: "-1",
    },
      h("span", { class: "tab-name" }, h("span", { class: "tab-num" }, i + 1), sc.name),
      h("span", { class: "tab-sub" }, sc.sub));
    tab.addEventListener("click", () => select(sc.id, true));
    tab.addEventListener("keydown", (e) => {
      const ids = SCENARIOS.map((s) => s.id);
      let j = null;
      if (e.key === "ArrowRight") j = (i + 1) % ids.length;
      else if (e.key === "ArrowLeft") j = (i - 1 + ids.length) % ids.length;
      else if (e.key === "Home") j = 0;
      else if (e.key === "End") j = ids.length - 1;
      if (j === null) return;
      e.preventDefault();
      select(ids[j], true);
      document.getElementById(`tab-${ids[j]}`).focus();
    });
    tablist.append(tab);
    const p = buildPanel(sc, i);
    panels.set(sc.id, p);
    panelsEl.append(p.panel);
  });
}

let currentId = null;

function select(id, push) {
  if (!panels.has(id)) id = SCENARIOS[0].id;
  if (push && location.hash !== `#${id}`) history.replaceState(null, "", `#${id}`);
  if (currentId === id) return;
  currentId = id;
  for (const [pid, p] of panels) {
    const on = pid === id;
    p.panel.hidden = !on;
    const tab = document.getElementById(`tab-${pid}`);
    tab.setAttribute("aria-selected", String(on));
    tab.tabIndex = on ? 0 : -1;
    if (on) tab.scrollIntoView({ block: "nearest", inline: "nearest" });
  }
  const p = panels.get(id);
  if (ready && !p.sc.async && !p.computed) { p.compute(); p.computed = true; }
}

window.addEventListener("hashchange", () => {
  const id = location.hash.slice(1);
  if (panels.has(id)) {
    select(id, false);
    document.querySelector(".tabs-wrap").scrollIntoView({ block: "start", behavior: "smooth" });
  }
});

// ---------- theme ----------

function setupTheme() {
  const btn = document.getElementById("theme-toggle");
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const effective = () => document.documentElement.dataset.theme || (media.matches ? "dark" : "light");
  const label = () => btn.setAttribute("aria-label", effective() === "dark" ? "Use light theme" : "Use dark theme");
  btn.addEventListener("click", () => {
    const next = effective() === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    try { localStorage.setItem("monadz-theme", next); } catch { /* storage can be blocked */ }
    label();
  });
  media.addEventListener("change", label);
  label();
}

// ---------- boot ----------

const statusEl = document.getElementById("status");

function showStatus(title, detail, error) {
  statusEl.classList.toggle("status-error", !!error);
  statusEl.setAttribute("role", error ? "alert" : "status");
  clear(statusEl, h("div", { class: "status-spinner", "aria-hidden": "true" }),
    h("div", {}, h("strong", {}, title), h("span", { class: "status-detail" }, " ", detail)));
}

async function fetchWithProgress(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
  const total = Number(res.headers.get("content-length")) || 0;
  if (!res.body || !res.body.getReader) return res.arrayBuffer();
  const reader = res.body.getReader();
  const chunks = [];
  let got = 0;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    chunks.push(value);
    got += value.length;
    const mb = (n) => (n / 1048576).toFixed(1);
    showStatus("Loading the Go library…", total ? `${mb(got)} of ${mb(total)} MB` : `${mb(got)} MB`);
  }
  const buf = new Uint8Array(got);
  let off = 0;
  for (const c of chunks) { buf.set(c, off); off += c.length; }
  return buf.buffer;
}

async function boot() {
  setupTheme();
  buildTabs();
  select(location.hash.slice(1), false);

  if (typeof WebAssembly !== "object" || typeof WebAssembly.instantiate !== "function") {
    showStatus("This browser cannot run WebAssembly.", "The playground runs the Go library as WebAssembly. Use a current version of Chrome, Firefox, Safari, or Edge.", true);
    return;
  }
  if (typeof window.Go !== "function") {
    showStatus("The Go loader (wasm_exec.js) did not load.", "Reload the page. If the error stays, the site is missing a file.", true);
    return;
  }
  try {
    const go = new window.Go();
    const isReady = new Promise((resolve) => { window.onMonadzReady = resolve; });
    const bytes = await fetchWithProgress("monadz.wasm");
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance).then(() => {
      ready = false;
      showStatus("The Go program stopped.", "Reload the page to start it again.", true);
    });
    await isReady;
  } catch (err) {
    showStatus("The Go library did not load.", String(err && err.message || err), true);
    return;
  }
  ready = true;
  document.getElementById("go-version").textContent = window.monadz.version;
  statusEl.classList.add("is-hidden");
  const p = panels.get(currentId);
  if (p && !p.sc.async) { p.compute(); p.computed = true; }
}

boot();
