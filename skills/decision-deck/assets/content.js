// The deck content: DECK, the slides (P), the small fixes (SMALL) and what was checked but needs no
// change (CHECKED). shot() and hl() come from index.html, and the helpers in visuals.js are available too.
const DECK = {
  title: "Project — decisions",
  // Answers persist in localStorage under this key. Use a new key per deck.
  storageKey: "project-decisions-v1",
  // Hue (0-360) per kind, used for the badge colour.
  kinds: { fix: { label: "Fix", hue: 3 }, change: { label: "Change", hue: 215 } },
};

// One slide per proposal, numbered in this order. The first option of each variant is the
// one you propose; it is preselected and labelled "(proposed)".
const P = [
{
  id: "p1", kind: "fix", title: "Example: the save button says what it does",
  problem: ["The button reads <b>Submit</b> on a screen that only saves a draft."],
  proposal: ["Rename it to <b>Save draft</b>.", "Show <i>Saved</i> for two seconds after it succeeds."],
  refs: "src/screens/Editor.tsx",
  variants: [{ key: "confirm", q: "How should a save be confirmed?", options: [
    ["inline", "Inline “Saved” label", "Quiet, next to the button."],
    ["toast", "Toast", "More visible, covers content briefly."],
  ]}],
  shots: [
    shot("now", "Current editor", card(["Title", `<span class="v-muted">Body…</span>`],
      `<div style="padding:16px"><span class="v-btn">Submit</span></div>`)),
    shot("new", "Inline confirmation", card(["Title", `<span class="v-muted">Body…</span>`],
      `<div style="padding:16px;display:flex;align-items:center;gap:12px">${hl("renamed", `<span class="v-btn">Save draft</span>`, "", "border-radius:6px")}${hl("new", `<span class="v-muted">Saved</span>`, "inline", "border-radius:4px")}</div>`),
      { confirm: "inline" }),
    shot("new", "Toast confirmation", card(["Title", `<span class="v-muted">Body…</span>`],
      `<div style="padding:16px 16px 72px">${hl("renamed", `<span class="v-btn">Save draft</span>`, "inline", "border-radius:6px")}</div>
      ${hl("new", `<div class="v-toast">Draft saved</div>`, "abs", "left:16px;right:16px;bottom:14px;border-radius:8px")}`),
      { confirm: "toast" }),
  ],
},
{
  id: "p2", kind: "change", title: "Example: retries move out of the request handler",
  problem: ["While the upstream API is slow, the handler retries it and keeps the client's request open."],
  proposal: ["Queue the call and retry it in a background worker.", "The handler answers 202 right away."],
  refs: "src/api/handler.ts · src/jobs/",
  shots: [
    shot("now", "Request path", `<div class="v-flow"><div class="v-box">Client</div>→<div class="v-box">Handler<br><span class="v-muted">retries ×3</span></div>→<div class="v-box">Upstream API</div></div>`),
    shot("new", "Request path", `<div class="v-flow"><div class="v-box">Client</div>→<div class="v-box">Handler</div>→${hl("new", `<div class="v-box">Queue</div>`, "", "border-radius:10px")}→${hl("new", `<div class="v-box">Worker<br><span class="v-muted">retries ×3</span></div>`, "", "border-radius:10px")}→<div class="v-box">Upstream API</div></div>`),
  ],
},
];

// Small, self-explanatory fixes on one slide, each decided separately. Leave empty to hide the slide.
const SMALL = [
  { id: "s1", title: "Example small fix", text: "One or two sentences on the problem and the fix. (file.ts)" },
];

// What you looked at and would leave as it is, each with a one-line reason. The user can add a note
// to anything they'd change anyway. Leave empty to hide the slide.
const CHECKED = [
  { id: "c1", title: "Example: the editor's autosave interval", why: "Every 10 seconds already; nobody reported losing text. (src/screens/Editor.tsx)" },
];
