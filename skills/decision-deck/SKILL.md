---
name: decision-deck
description: Work out the decisions the user has to make (proposed changes, review findings, a choice between designs, approaches or trade-offs, or open questions) and present them as an interactive slide deck, with visuals where they help. Use when the user wants options shown visually (a presentation, mockups, diagrams, a deck) to decide on before anything is implemented.
---

## 1. Work out the decisions

Start from whatever the user gave you. It may be proposals or options already discussed in the conversation, or only a goal, area or open question, such as "find UX issues in the app", "propose how to split this module" or "which storage should we use for sessions?". In that case, investigate it yourself: read the relevant code, find the problems or the possible answers, and decide what you would recommend. Don't post the list in chat first; the deck is where you present it.

In the project's `.agents-workspaces/`, read only `deck-kit/` and the workspace you create for this deck. Other workspaces, their documents and their decks included, belong to other tasks: ask the user before opening anything in them.

Each slide is one decision: a proposed change, a finding to act on, or a question. Title it with the outcome the user gets, or the question being decided, not the mechanism behind it. Shape the slide's content to the decision: what is wrong or why a decision is needed, verified against the code; what you recommend; and the question, with the controls that let the user decide it. Pick controls the way the answers combine. Radios when exactly one answer applies, checkboxes when each option is worth taking or leaving, a text or number field for a name or a threshold, a select or slider for a long or continuous range. Give the recommended answer first and make it the proposed one.

Word every proposal as a change to make, so Keep means "do it" and Drop means "don't". An item that says what won't happen turns Drop into a double negative. Implementation details belong in the plan, not the deck.

Before building the deck, check that the proposals fit together and fit the app as it is. A slide that depends on an earlier slide's choice says which option it assumes. The recommended answers, the small fixes and the later slides must work together. New UI goes on screens and menus that exist, or the slide proposes the new screen and shows how the user gets to it.

Everything you looked at ends up in the deck: as a slide, as a row in a slide of small fixes (each row gets its own decision), or in a "checked" slide with the reason it stays. The user can add a note to a checked item to change it anyway. If the scope is so large that one deck would be tiring to go through, build several complete decks, one per area. Order the slides so each decision comes before the ones that depend on it.

## 2. Build the deck

### Files
1. Create a workspace for the decisions with the `create-agent-workspace` skill and put the deck in its `deck/` directory. If that skill isn't available, or the deck is a throwaway, use `mktemp -d /tmp/<slug>-deck-XXXX`. Never write the deck into the project tree.
2. Copy [assets/deck.html](assets/deck.html) there as `index.html`. It is the deck's engine and its own look; don't edit it. Copy [assets/visuals.css](assets/visuals.css), [assets/visuals.js](assets/visuals.js) and [assets/content.js](assets/content.js) next to it, and replace the examples in them with your own.
3. If the project has a deck kit, `.agents-workspaces/deck-kit/`, start from it: copy its `visuals.css`, `visuals.js` and assets, and read its `KIT.md`. It names the project files and the commit the kit was taken from. If those files changed since then for screens this deck draws (`git log <commit>..HEAD -- <files>`), update the kit's styles and helpers from them first.

### content.js
- `DECK`: `title`, a new `storageKey` per deck (so answers from another deck don't load), and optionally `kinds`, a badge and hue (0–360) per slide `kind`.
- `SLIDES`: an array of `{ id, title, kind?, body, decision? }`, shown in array order.
  - `body` is any HTML string: a layout, visuals, controls, and scripts. `shot()` and `hl()` are available to build visuals.
  - `decision: false` leaves out the Keep / Change / Drop bar. Use it for an open question, or for a slide whose decisions are all `data-decision` rows.
  - `id` starts with a letter and uses only letters, digits, `-` and `_`. It is unique in the deck and is not `summary`.

### Answers
The deck reads these attributes from the body. Nothing else is needed for an answer to be saved and exported.

- **A question**: `data-answer="key"` on each control that answers it. The key is unique within its slide. Put `data-label="the question"` on the first control of the group.
  - Radios with the same key are one question; only one option is chosen. Mark the proposed option with `checked` and put it first.
  - Checkboxes with the same key are a set. Check the proposed ones.
  - `select`, `input` (`text`, `number`, `range`, and the rest) and `textarea` are values. Set the proposed value in the markup.
- **A custom widget**, such as a slider or drag-to-rank: put `<input type="hidden" data-answer="key" data-label="...">` in the body. The widget sets its `value` and dispatches a bubbling `input` event. Scripts in a body run after the saved answers are back, so they can read them. A script finds its own slide with `document.currentScript.closest(".dk-slide")`.
- **Option text**: `data-option="Label"` on a radio, checkbox or select option sets the text the export shows. Without it, the export uses the wrapping label's text (or a select option's own text), then the value.
- **Per-element decisions**: `data-decision="id" data-label="..."` on any element gives it Keep / Change / Drop and a note box. Use it for small fixes, rows in a list, or checked items you want decided. Ids are unique in the deck.
- **Notes**: a slide with a bar has a note box. A `textarea` with `data-answer` is a note that the export lists as an answer, which suits checked items.
- **Showing parts**: `data-when="key=value"` on an element shows it only while the slide's answer for `key` is `value`. For a set, it shows while that box is checked. `shot()`'s last argument does the same for a whole visual.
- A slide set to Change with an empty note leaves its note box highlighted, to ask for the change.

### Layout and helpers
- `<div class="dk-split"><div class="dk-text">…</div><div class="dk-stage"><div class="dk-shots">…</div></div></div>` puts text on the left and visuals on the right. This is the default for a proposal, but it isn't required. Visuals inside `.dk-shots` are scaled to fit `.dk-stage`, up to 140%. Don't cap a visual with `overflow: hidden`: the deck can't see what the cap hides.
- Other classes for body markup: `dk-q` (question label), `dk-option` (an option row, with `<span>` text and an `<i>` hint), `dk-hint`, `dk-pick` (a pill button; the deck styles `aria-pressed="true"`), `dk-item` (a card). The `dk-` prefix belongs to the deck, so the visuals don't use it.
- `shot(kind, caption, html, when)`: `kind` is `"now"` for the current state or `"new"` for the proposal. Skip "now" when there is no current state to compare against. `when` is an answer object such as `{ layout: "grid" }`.
- `hl(tag, html, cls, style)`: a dashed outline and a short tag around a changed part of a proposed visual, such as "new" or "renamed". Use `inline` for a label inside a row or a cell. Use `abs` for an overlay: put its position in `style` and give the inner element `position: static`. The tag sits above the outline's top-right corner, so leave room above it, or an ancestor with `overflow: hidden` clips it.

### Visuals
- A mockup of an existing UI looks like that UI: its colours, fonts and components from the project's styles, its real labels and copy, and its own assets (icon fonts, SVGs, images) copied next to `index.html` rather than redrawn. An icon font needs each glyph's code point, usually from the glyph map (JSON) shipped with the font.
- Draw the app's default theme. Show the other theme only when the change looks different in it. Use realistic sample content.
- You may use real screenshots of the current state for "now" shots, at the mockups' scale: a screenshot's CSS width is its pixel width divided by the screen density (on Android, `adb shell wm density` ÷ 160). Capture only states you can reach without changing the user's data: don't send messages, save settings, or create, edit or delete anything to get there. Draw those states instead.
- Fonts: the browser may not have the platform font the app uses, such as Roboto or SF Pro, and falls back silently. Copy the font file next to `index.html` (from the project, its dependencies, or a device: `adb pull /system/fonts/Roboto-Regular.ttf`) and load it with `@font-face`. End every font stack with a generic family.
- Diagrams, code and tables are plain HTML and CSS. Break long code lines, or let them wrap with `white-space: pre-wrap`.
- Keep a slide to about three visuals side by side. More makes them too small to read. Show several states of one screen as a grid inside one visual, or split the slide.
- `visuals.css` holds the app's look for mockups. Give each mockup its own background and text colour, so it looks the same in both themes. Diagrams, code and tables may follow the theme with `--page-card`, `--page-text` and `--page-muted`, or with rules under `[data-theme=dark]`.
- `visuals.js` holds helpers for parts the mockups repeat, such as a phone frame, an app bar or a list row, so each slide's markup stays short and consistent.

## 3. Check it renders

1. Pick a free port (check with `ss -ltn`) and run `python3 -m http.server <port> --bind 127.0.0.1` in the background from the deck directory.
2. Run [scripts/render.mjs](scripts/render.mjs): `node <this skill's directory>/scripts/render.mjs <deck URL> <out dir>`. It needs Node 22+ and a Chromium-family browser (set `DECK_BROWSER` if it isn't found). It screenshots every slide in its proposed state, then each option of every radio group, each value of every select, and each box of every checkbox set, with the other answers reset to their proposed state. Text, number and range fields are only shown as proposed. For each shot it prints the scale the visuals were fitted to, and it lists visuals that clip their content with `overflow: hidden`. Under a red "Deck error" bar, a script error or a file that failed to load is named with its file and line. Read the screenshots and fix what looks wrong: misaligned or cramped visuals, truncated text, clipped highlight tags, and overlays stretched full width or misplaced. Then run it again with `DECK_THEME=dark` into another directory and check that no text in a visual has become unreadable. Visuals scaled below about 70% are too crowded. Run it again after each round of fixes.
   To test specific answers, pass a steps JSON as the third argument: `[{ "name", "hash", "eval" }]`. `hash` is the slide number, and `eval` is JavaScript whose result is printed. `__dkPick(i, "key", j)` clicks the j-th option (or sets a select to it), and `__dkReset(i)` puts slide `i` (from 0) back to its proposed answers. `document.getElementById("exportText").value` holds the export on the last slide.
3. Use the script even when you have a browser preview; preview screenshots are too blurry to show small text. Only if the script can't run, check in the preview: step through each slide with `location.hash = "#N"`, change each answer once, and wait for `document.fonts.ready` before judging fonts and icons.
4. Open the deck in the user's browser preview, if you have one, at the size it opens. Don't resize the preview or change its viewport. The deck fits its visuals to the window by itself, and resizing has made previews stop responding.
5. Test the controls once: arrow keys move slides, a decision colours its dot, and the summary lists the answers. Then clear your test answers with `localStorage.removeItem(DECK.storageKey)` in the preview, and load the page again. Don't call `location.reload()` in a scripted evaluate.

If you couldn't render the page at all, say so when you hand it over.

## 4. Hand it over

Give the user the URL and open it in their preview if you can. Keep the message short, about five lines. The deck is where the proposals are, and its last slide lists every slide, so don't list, summarise or tabulate the slides in chat, or suggest an order to go through them. Say:

- How many slides there are.
- Anything the user needs to know before reviewing that the slides don't make clear, such as mockups being drawn rather than captured, made-up sample data, or a bug you reproduced while investigating. If there's nothing, skip this.
- In one line: ← and → move between slides; a slide with a bar is decided with Keep / Change / Drop; each slide is answered with its own controls and notes; the last slide exports the answers as text to paste back.
- Where the deck lives, that nothing in the project has changed yet, and that the local server is still running. If you couldn't open it in their preview, say so.

Keep the server running while the user reviews. Stop it when they're done, or when you finish the follow-up work.

While the user reviews:

- Fill in the workspace's documents: the request, what you found, and where the deck is.
- If the deck drew the app's UI and lives in a workspace, save the project's look for the next deck. Copy `visuals.css`, `visuals.js` and the assets they use (fonts, icon fonts and glyph maps, icons) to `.agents-workspaces/deck-kit/`, creating it or updating what you changed. Keep only what draws the app, not this deck's mockups or screenshots. Write or update its `KIT.md`: what each helper draws, the project files and commit the look was taken from, where the fonts and icons came from, and how you captured screenshots of the app, if you did.

## 5. Act on the decisions

The export is plain text, one block per slide in order:

- A header `N. Title: KEEP | CHANGE | DROP | UNDECIDED`. A slide without a bar has the title only.
- Each answer as `label: value`. A set lists each checked option on its own line, or `(none)` when nothing is checked. A blank value is `(empty)`.
- Keep and Change list every answer, untouched ones at their proposed values. Drop, Undecided and slides without a bar list only the answers the user changed.
- A `Note:` line for a slide's note. Each `data-decision` element is listed as `- label: DECISION`, with its note.

Read each decision like this:

- **KEEP**: the user accepts the slide as shown, with the answers given.
- **CHANGE**: the user accepts it with the changes in the note. If the note is missing or leaves anything unclear, ask about that slide.
- **DROP**: rejected. Don't implement it and don't propose it again. A note may say what to do instead; treat that as the decision, and ask if it's unclear.
- **UNDECIDED**: still open. Ask about it.
- A checked item with a note: the user wants it changed after all. Ask, or propose it in a follow-up deck.

Reply in chat with what you understood: the accepted items with their answers and notes, the dropped ones, and your questions about unclear notes and undecided items. Check that each accepted item is actually possible (for example, that a dependency or API it relies on supports it) and say which ones aren't. Then stop. Make no changes to the project until the user explicitly tells you to implement.

When notes ask to see something again or for other options ("try again", "show it with more rows"), or the user asks for another pass, answer with the deck, not with options in chat. Add the new slides or answers to the same deck, keeping existing slide and answer ids unchanged so earlier answers stay. Or build a follow-up deck next to it: copy `index.html` to `round2.html`, point its `content.js` script at `round2.js`, and put only the reopened and new slides in that file, with a new `storageKey`. It reuses `visuals.css`, `visuals.js` and the assets. Check it renders and hand it over as above.
