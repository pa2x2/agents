---
name: decision-deck
description: Work out the decisions the user has to make (proposed changes, review findings, a choice between designs, approaches or trade-offs) and present them as an interactive slide deck with before/after or side-by-side visuals, where the user decides Keep / Change / Drop per item, picks variants, adds notes, and pastes the exported decisions back. Use when the user wants options shown visually (a presentation, mockups, diagrams, a deck) to decide on before anything is implemented.
---

## 1. Work out the decisions

Start from whatever the user gave you. It may be proposals or options already discussed in the conversation, or only a goal, area or open question, such as "find UX issues in the app", "propose how to split this module" or "which storage should we use for sessions?". In that case, investigate it yourself: read the relevant code, find the problems or the possible answers, and decide what you would recommend. Don't post the list in chat first; the deck is where you present it.

Each slide is one decision: a proposed change, a finding to act on, or a question with several reasonable answers. Shape each for a slide:

- **Title**: the outcome the user gets, or the question being decided, not the mechanism behind it.
- **Kind**: a short category label. Choose a few that fit this set of decisions and give each a hue in `DECK.kinds`.
- **Problem**: what is wrong today, or why a decision is needed, verified against the code. Two to five bullets.
- **Proposal**: what you recommend. Two to five bullets. For a choice between options, describe what they have in common here and let the variants carry the differences.
- **Refs**: the files involved.
- **Variants**: only where you see more than one reasonable answer. Put the one you recommend first; the deck preselects it and labels it "(proposed)". Each option gets a short hint on its trade-off.
- **Shots**: one or more "Now" visuals, then one "Proposed" visual per variant or per state worth showing. Skip "Now" when there is no current state to compare against, such as a choice for something not built yet. Tie each variant's visual to its option with `shot()`'s last argument, e.g. `{ layout: "grid" }`. The slide then shows only the selected option's visual, and switching the option swaps it.

Group related findings into one slide. Put small, self-explanatory fixes in the `SMALL` list, which all share one slide and are decided one by one.

Word every slide and small fix as a change to make, so Keep means "do it" and Drop means "don't". An item that says what won't happen ("No new tests", "Debug builds have no updater") turns Drop into a double negative. Implementation details like that belong in the plan, not the deck; if one needs a decision, word it as the action ("Leave the updater out of debug builds").

## 2. Create the deck

1. Create a workspace for the decisions with the `create-agent-workspace` skill and put the deck in a `deck/` directory inside it. If that skill isn't available, or the deck is a throwaway (a test run, a one-off the user won't keep), put everything in a new directory under `/tmp` instead (`mktemp -d /tmp/<slug>-deck-XXXX`). Never write the deck into the project tree.
2. Copy [assets/deck.html](assets/deck.html) there as `index.html`. Only the two sections marked `EDIT` need changes: visual styles and deck content (`DECK`, `P`, `SMALL`). The deck itself keeps its neutral look. Delete the example slides. Set a new `DECK.storageKey` so answers from another deck don't load. The deck's own classes all start with `dk-`; don't use that prefix in visuals.
3. Draw each shot in the form that shows the change best. `shot()` takes any HTML:
   - **UI changes**: a mockup of the screen or component. Make it look like the real UI: its colours, fonts and components from the project's styles, its real labels and copy, and its own assets (icon fonts, SVGs, images) copied next to `index.html` rather than redrawn. An icon font needs each glyph's code point, which is usually in a glyph map (JSON) shipped with the font's package. Draw the app's default theme; show the other theme too only when the change looks different in it. Use realistic sample content. If you can capture real screenshots of the current state, you may use them as `<img>` in the "Now" shots. Show them at the mockups' scale so both sides compare at the same size: with mockups drawn at one CSS pixel per dp or pt, a screenshot's CSS width is its pixel width divided by the screen density (on Android, `adb shell wm density` ÷ 160; 1080 px at 420 dpi → 411 px). Capture only states you can reach without changing the user's data: don't send messages, save settings, or create, edit or delete anything to get there. Draw those states instead.
   - **Fonts**: the browser may not have a platform font the app uses, such as Roboto on Android or SF Pro on Apple platforms, and silently falls back to another face. Copy the font file next to `index.html` (from the project, its dependencies, or a device or emulator: `adb pull /system/fonts/Roboto-Regular.ttf`) and load it with `@font-face`. End every font stack with a generic family (`sans-serif`).
   - **Anything else**: a diagram of the flow or structure, before/after code or config, a table, sample output. Plain HTML and CSS is enough.

   Keep a slide to about three visuals side by side. The deck scales the visuals to fit the window, enlarging small ones up to 140% and shrinking big ones as far as needed, so more makes them too small to read. Don't add your own zoom to them. Show several states of one screen as a grid inside one visual, or split the slide.
4. Wrap each changed part of a Proposed shot in `hl(tag, html, cls, style)`, so a dashed outline and a short tag ("new", "renamed", "appears on edit") mark what changes. Leave "Now" shots unmarked. By default `hl` is a block `<div>`. Two classes change that:
   - `inline`: an inline-block `<span>`, for a label inside a row or a value in a table cell.
   - `abs`: for an overlay, popover or dialog. The wrapper becomes the absolutely positioned box, so pass its position (`top`, `right`, ...) in `style` and give the inner element `position: static`.

   The tag sits above the outline's top-right corner, outside the wrapped element. An ancestor with `overflow: hidden`, such as a phone frame or a rounded card, clips it, and outlines close together overlap their tags. Leave room above each outline, or put the `overflow: hidden` inside the wrapped part.
5. Use `DECK.intro` for the suggested order to go through the slides and for caveats, such as UI mockups being drawn rather than captured.

## 3. Check it renders

Serve the directory and look at every slide before handing it over.

1. Pick a free port (check with `ss -ltn`; a stale server may already hold a common one) and run `python3 -m http.server <port> --bind 127.0.0.1` in the background from the deck directory.
2. Run [scripts/render.mjs](scripts/render.mjs): `node <this skill's directory>/scripts/render.mjs <deck URL> <out dir>`. It needs Node 22+ and a Chromium-family browser (set `DECK_BROWSER` if it isn't found). It screenshots every slide at full resolution, and every option of every variant with the slide's other variants on their proposed option. For each shot it prints the scale the visuals were fitted to, and it lists any page errors. Read the screenshots and fix what looks wrong: misaligned or cramped visuals, truncated text, clipped highlight tags, and overlays stretched full width or misplaced (see `abs` above). Visuals scaled below about 70% are too crowded. A red "Deck error" bar instead of slides means the deck script has an error at the line it names. Run it again after each round of fixes.

   Use the script even when you have a browser preview: preview screenshots are usually scaled down and too blurry to read small text, so they miss what this check is for. Only if the script can't run, check in the preview instead: step through each slide (set `location.hash = "#N"` or click `.dk-dots [data-go="N"]`), select each variant option once, and wait for `document.fonts.ready` before judging fonts and icons.
3. Open the deck in the user's browser preview, if you have one, at the size it opens. Don't resize the preview or change its viewport. The deck fits its visuals to the window by itself, and resizing has made previews stop responding. If a preview call fails, open the page in it again (visibly, if the tool can show or hide it) and retry once before deciding it doesn't work.
4. Test the controls once: arrow keys move slides, a decision colours its dot, and the summary slide lists the answers. In the preview, clear the test answers afterwards with `localStorage.removeItem(DECK.storageKey)`, then load the page again with `location.href = "<deck URL>#0"` or the preview's own navigation. Don't call `location.reload()` inside a scripted evaluate: the call fails when the page goes away under it. Without a preview, test them with the script's own steps (its header explains them); answers set that way stay in its throwaway browser profile.

If you couldn't render the page at all, say so when you hand it over.

## 4. Hand it over

Give the user the URL and open it in their preview if you can. Keep the message short, about five lines. The deck is where the proposals are, and its intro lists every slide, so don't list, summarise or tabulate the slides in chat. Say:

- How many slides there are.
- Anything the user needs to know before reviewing that the slides don't make clear, such as mockups being drawn rather than captured, made-up sample data, or a bug you reproduced while investigating. If there's nothing, skip this.
- In one line: ← and → move between slides, each slide is decided with Keep / Change / Drop, variants and notes, and the last slide exports the answers to paste back.
- Where the deck lives, that nothing in the project has changed yet, and that the local server is still running. If you couldn't open it in their preview, say so.

Keep the server running while the user reviews. Stop it when they're done, or when you finish the follow-up work.

## 5. Act on the decisions

The pasted export records the user's decisions. It is not permission to implement anything. Read each decision like this:

- **KEEP**: the user accepts the slide's proposal as shown, with the chosen variant.
- **CHANGE**: the user accepts it with the changes in their note. If the note is missing or leaves anything unclear, ask about that slide.
- **DROP**: rejected. Don't implement it and don't propose it again. A note on it may say what to do instead ("use the icon the app already has"); treat that as the decision, and ask if it's unclear.
- **UNDECIDED**: still open. Ask about it.

Under DROP and UNDECIDED, a listed variant is one the user picked; untouched variants are left out.

Reply in chat with what you understood: the accepted items with their variants and notes, the dropped ones, and your questions about unclear notes and undecided items. Check that each accepted item is actually possible (for example, that a dependency or API it relies on supports it) and say which ones aren't. Then stop. Make no changes to the project until the user explicitly tells you to implement.

When notes ask to see something again or for other options ("try again", "show it with more rows"), or the user asks for another pass, answer with the deck, not with options in chat. Add the new slides or variants to the same deck, keeping slide ids unchanged so earlier answers stay. Or build a follow-up deck next to it (for example `round2.html`) that reuses its styles and assets, has a new `storageKey`, and holds only the reopened and new items. Check it renders and hand it over as above.
