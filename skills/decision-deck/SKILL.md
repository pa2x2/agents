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

## 2. Create the deck

1. Create a workspace for the decisions with the `create-agent-workspace` skill and put the deck in a `deck/` directory inside it. If that skill isn't available, or the deck is a throwaway (a test run, a one-off the user won't keep), put everything in a new directory under `/tmp` instead (`mktemp -d /tmp/<slug>-deck-XXXX`). Never write the deck into the project tree.
2. Copy [assets/deck.html](assets/deck.html) there as `index.html`. Only the two sections marked `EDIT` need changes: visual styles and deck content (`DECK`, `P`, `SMALL`). The deck itself keeps its neutral look. Delete the example slides. Set a new `DECK.storageKey` so answers from another deck don't load.
3. Draw each shot in the form that shows the change best. `shot()` takes any HTML:
   - **UI changes**: a mockup of the screen or component. Make it look like the real UI: its colours, fonts and components from the project's styles, its real labels and copy, and its own assets (icon fonts, SVGs, images) copied next to `index.html` rather than redrawn. An icon font needs each glyph's code point, which is usually in a glyph map (JSON) shipped with the font's package. Draw the app's default theme; show the other theme too only when the change looks different in it. Use realistic sample content. If you can capture real screenshots of the current state, you may use them as `<img>` in the "Now" shots.
   - **Anything else**: a diagram of the flow or structure, before/after code or config, a table, sample output. Plain HTML and CSS is enough.
4. Wrap each changed part of a Proposed shot in `hl(tag, html, cls, style)`, so a dashed outline and a short tag ("new", "renamed", "appears on edit") mark what changes. Leave "Now" shots unmarked. By default `hl` is a block `<div>`. Two classes change that:
   - `inline`: an inline-block `<span>`, for a label inside a row or a value in a table cell.
   - `abs`: for an overlay, popover or dialog. The wrapper becomes the absolutely positioned box, so pass its position (`top`, `right`, ...) in `style` and give the inner element `position: static`.
5. Use `DECK.intro` for the suggested order to go through the slides and for caveats, such as UI mockups being drawn rather than captured.

## 3. Check it renders

Serve the directory and look at every slide before handing it over.

1. Pick a free port (check with `ss -ltn`; a stale server may already hold a common one) and run `python3 -m http.server <port> --bind 127.0.0.1` in the background from the deck directory.
2. Open it in whatever browser preview or screenshot tool you have. Wait for `document.fonts.ready` before judging fonts and icons.
3. Step through each slide (set `location.hash = "#N"`, press → or click `.dots [data-go="N"]`) and fix what looks wrong: misaligned or cramped visuals, truncated text, overlays stretched full width or misplaced (see `abs` above), and visual class names colliding with deck classes (prefix them with `v-`). On slides with variants, select each option once to check its visual.
4. Test the controls once: arrow keys move slides, a decision colours its dot, and the summary slide lists the answers. Clear the test answers from `localStorage` afterwards.

If you have no way to render the page, say so when you hand it over.

## 4. Hand it over

Give the user the URL and open it in their preview if you can. In a few lines, explain:

- ← and → or the dots move between slides; the intro lists every slide.
- Each slide has Keep / Change / Drop, variant choices (picking one shows its visual), and a notes box. Answers are saved in the browser.
- "Highlight changes" turns the dashed outlines off.
- The last slide ("Export decisions") has the text to copy and paste back.
- Where the deck lives, that nothing in the project has changed yet, and that the local server is still running.

Keep the server running while the user reviews. Stop it when they're done, or when you finish the follow-up work.

## 5. Act on the decisions

The pasted export records the user's decisions. It is not permission to implement anything. Read each decision like this:

- **KEEP**: the user accepts the slide's proposal as shown, with the chosen variant.
- **CHANGE**: the user accepts it with the changes in their note. If the note is missing or leaves anything unclear, ask about that slide.
- **DROP**: rejected. Don't implement it and don't propose it again.
- **UNDECIDED**: still open. Ask about it. A variant listed under it is one the user picked; untouched variants are left out.

Reply in chat with what you understood: the accepted items with their variants and notes, the dropped ones, and your questions about unclear notes and undecided items. Check that each accepted item is actually possible (for example, that a dependency or API it relies on supports it) and say which ones aren't. Then stop. Make no changes to the project until the user explicitly tells you to implement.
