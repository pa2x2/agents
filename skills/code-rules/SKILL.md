---
name: code-rules
description: Code writing guidelines that must be followed whenever writing code, including when writing, keeping or reviewing code comments.
---

## Design

- One code path for one piece of logic. No duplicated models, mappings or "changed here, forgot there" splits.
- If the feature/changes hint at future variants, make the design extensible without new if branches (for example, register handlers through DI).
- No speculative fields, options or config the feature/change doesn't need.
- Do it the way sibling services already do it. Don't introduce config or patterns that no other service uses unless absolutely necessary.
- Remove what the change replaces (old contracts, endpoints, helpers).
- Put a new operation on the existing service that owns its data. Don't add a service and interface just to hold one method.
- No enum or type whose only job is to map one-to-one onto another. Use the original directly. A local type is justified only when one member stands for a combination of values.
- Use typed models, not dynamic JSON or dictionaries. Keep dynamic structures for data that is genuinely schema-less.
- Name things in the platform's own vocabulary, not a third-party library's internals. Keep upstream names only where the contract requires them (wire names, shapes).
- Reuse the existing shared component, hook or helper instead of writing a local one. If it falls short, ask before building an alternative or working around it.

## Fixes

- Fix the cause, not the symptom. Find why the issue happens before changing code; no band-aid fixes.
- Don't leave warnings a change introduces. Fix what causes them instead of suppressing them or disabling warnings-as-errors.

## Comments

A comment earns its place by telling the reader something the code can't: why it's done this way, a constraint or platform quirk it works around, a bug it prevents, or what a non-obvious value means. If deleting the comment loses nothing a careful reader couldn't get from the code, delete it.

- Don't restate the code. No /** Cancels a running download. */ over cancelDownload, no // Refresh on resume over an onResume override, no /** Last failure, as a user-facing message. */ over error: String?.
- Don't narrate names or types. If a property, parameter or function name plus its type already says it, leave it bare. Document a property only when its meaning isn't obvious: units, what null stands for, who sets it, what it must never be.
- Don't write file headers that only name the file. "Tests for the library repository" or "Chapter list screen model" add nothing. A header is worth it when it explains a design: how the parts fit, what the module owns, a contract callers rely on.
- Keep comments true. When you change code, update or delete the comments it touches. A stale comment is worse than none.
