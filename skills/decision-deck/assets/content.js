// The deck content: DECK (title, storage key, badge kinds) and SLIDES, one per decision in the order to
// decide them. shot() and hl() come from index.html, and the helpers in visuals.js are available too.
// The markup a body uses for answers is described in SKILL.md.
const DECK = {
  title: "Project — decisions",
  // Answers persist in localStorage under this key. Use a new key per deck.
  storageKey: "project-decisions-example",
  // Hue (0-360) per kind, used for the badge colour.
  kinds: { fix: { label: "Fix", hue: 3 }, change: { label: "Change", hue: 215 } },
};

const SLIDES = [
{
  id: "save", kind: "fix", title: "Example: the save button says what it does",
  body: `<div class="dk-split">
    <div class="dk-text">
      <h3>Problem</h3><ul><li>The button reads <b>Submit</b> on a screen that only saves a draft.</li></ul>
      <h3>Proposal</h3><ul><li>Rename it to <b>Save draft</b>.</li><li>Confirm the save the way chosen below.</li></ul>
      <div class="dk-refs">src/screens/Editor.tsx</div>
      <div class="dk-q">How should a save be confirmed?</div>
      <label class="dk-option"><input type="radio" data-answer="confirm" data-label="How should a save be confirmed?" data-option="Inline Saved label" value="inline" checked><span>Inline “Saved” label<i>Quiet, next to the button.</i></span></label>
      <label class="dk-option"><input type="radio" data-answer="confirm" data-option="Toast" value="toast"><span>Toast<i>More visible, covers content briefly.</i></span></label>
    </div>
    <div class="dk-stage"><div class="dk-shots">
      ${shot("now", "Current editor", `<div class="v-card"><div class="v-row">Title</div><div class="v-row"><span class="v-muted">Body…</span></div><div style="padding:16px"><span class="v-btn">Submit</span></div></div>`)}
      ${shot("new", "Inline confirmation", `<div class="v-card"><div class="v-row">Title</div><div class="v-row"><span class="v-muted">Body…</span></div><div style="padding:16px;display:flex;align-items:center;gap:12px">${hl("renamed", `<span class="v-btn">Save draft</span>`, "", "border-radius:6px")}${hl("new", `<span class="v-muted">Saved</span>`, "inline", "border-radius:4px")}</div></div>`, { confirm: "inline" })}
      ${shot("new", "Toast confirmation", `<div class="v-card"><div class="v-row">Title</div><div class="v-row"><span class="v-muted">Body…</span></div><div style="padding:16px 16px 72px">${hl("renamed", `<span class="v-btn">Save draft</span>`, "inline", "border-radius:6px")}</div>${hl("new", `<div class="v-toast">Draft saved</div>`, "abs", "left:16px;right:16px;bottom:14px;border-radius:8px")}</div>`, { confirm: "toast" })}
    </div></div>
  </div>`,
},
{
  id: "retry", kind: "change", title: "Example: retries move out of the request handler",
  body: `<div class="dk-split">
    <div class="dk-text">
      <h3>Problem</h3><ul><li>While the upstream API is slow, the handler retries it and keeps the client's request open.</li></ul>
      <h3>Proposal</h3><ul><li>Queue the call and retry it in a background worker.</li><li>The handler answers 202 right away.</li></ul>
      <div class="dk-refs">src/api/handler.ts · src/jobs/</div>
    </div>
    <div class="dk-stage"><div class="dk-shots">
      ${shot("now", "Request path", `<div class="v-flow"><div class="v-box">Client</div>→<div class="v-box">Handler<br><span class="v-muted">retries ×3</span></div>→<div class="v-box">Upstream API</div></div>`)}
      ${shot("new", "Request path", `<div class="v-flow"><div class="v-box">Client</div>→<div class="v-box">Handler</div>→${hl("new", `<div class="v-box">Queue</div>`, "", "border-radius:10px")}→${hl("new", `<div class="v-box">Worker<br><span class="v-muted">retries ×3</span></div>`, "", "border-radius:10px")}→<div class="v-box">Upstream API</div></div>`)}
    </div></div>
  </div>`,
},
{
  id: "row", kind: "change", title: "Example: the row shows the details you keep",
  body: `<div class="dk-split">
    <div class="dk-text">
      <div class="dk-q">Which details should the row show?</div>
      <label class="dk-option"><input type="checkbox" data-answer="cols" data-label="Which details should the row show?" data-option="Status" value="status" checked><span>Status<i>What people scan for.</i></span></label>
      <label class="dk-option"><input type="checkbox" data-answer="cols" data-option="Owner" value="owner" checked><span>Owner<i>Often empty on old rows.</i></span></label>
      <label class="dk-option"><input type="checkbox" data-answer="cols" data-option="Updated" value="updated"><span>Updated<i>Noisier, useful when the list is long.</i></span></label>
      <div class="dk-q">Row action label</div>
      <input type="text" data-answer="action" data-label="Row action label" value="Open">
      <div class="dk-hint">A verb, short enough to sit on the row.</div>
      <div class="dk-q">Row density</div>
      <input type="hidden" data-answer="density" data-label="Row density" value="compact">
      <div><button type="button" class="dk-pick" data-v="compact">Compact</button> <button type="button" class="dk-pick" data-v="comfortable">Comfortable</button></div>
      <script>
        (() => {
          const root = document.currentScript.closest(".dk-slide");
          const input = root.querySelector('input[data-answer="density"]');
          const buttons = root.querySelectorAll("[data-v]");
          const show = () => buttons.forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.v === input.value)));
          buttons.forEach((b) => b.addEventListener("click", () => {
            input.value = b.dataset.v;
            input.dispatchEvent(new Event("input", { bubbles: true }));
            show();
          }));
          show();
        })();
      </script>
    </div>
    <div class="dk-stage"><div class="dk-shots">
      ${shot("now", "Current row", `<div class="v-card"><div class="v-row">Title</div></div>`)}
      ${shot("new", "Details you keep, on the row", `<div class="v-card"><div class="v-row">Title</div>
        <div class="v-row" data-when="cols=status">${hl("new", "In progress", "inline", "border-radius:4px")}</div>
        <div class="v-row" data-when="cols=owner">${hl("new", "Ada", "inline", "border-radius:4px")}</div>
        <div class="v-row" data-when="cols=updated">${hl("new", "2 hours ago", "inline", "border-radius:4px")}</div>
        <div style="padding:16px"><span class="v-btn">Open</span></div></div>`)}
    </div></div>
  </div>`,
},
{
  id: "small", decision: false, title: "Small fixes",
  body: `<p class="dk-lead">Each fix is decided on its own.</p>
    <div data-decision="s1" data-label="Fix the typo in the empty state"><p><b>Fix the typo in the empty state.</b> “Nothign” → “Nothing” (src/screens/List.tsx)</p></div>
    <div data-decision="s2" data-label="Show the unsaved-changes warning on close"><p><b>Show the unsaved-changes warning on close.</b> It's only shown on back navigation today. (src/screens/Editor.tsx)</p></div>`,
},
{
  id: "checked", decision: false, title: "Checked, no change proposed",
  body: `<p class="dk-lead">Looked at while preparing this deck and left as it is, for the reason given. Add a note to anything you'd change anyway.</p>
    <div class="dk-item"><p><b>The editor's autosave interval.</b> Every 10 seconds already; nobody reported losing text. (src/screens/Editor.tsx)</p>
      <textarea data-answer="autosave" data-label="Editor autosave interval" placeholder="Notes"></textarea></div>`,
},
];
