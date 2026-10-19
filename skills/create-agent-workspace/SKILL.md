---
name: create-agent-workspace
description: Create, resume and maintain a workspace recording a feature's or change's requirements, rules, architecture, decisions, plan and dated notes. Use when the user asks to create a workspace or record decisions so far; asks to resume, continue, start working on or update a workspace, or whether it is up to date; or references a `.agents-workspaces/` path.
---

# Agent Workspace

A workspace is the durable record of one feature or change that later sessions resume from. Keep reporting to the user in chat; don't point them at files instead of answering.

Creating or editing a workspace doesn't authorize product changes, commits or external actions. When the user asks to start or continue work on a unit, that authorizes that unit.

## Create

1. Root: `git rev-parse --show-toplevel`, or the current directory outside Git.
2. Create `<root>/.agents-workspaces/<feature>/`: a kebab-case slug of the request. Never overwrite; use the next numeric sibling (`reader-refactor-2`).
3. Ensure `$(git rev-parse --git-common-dir)/info/exclude` contains `/.agents-workspaces/`. Don't edit tracked files such as `.gitignore`.
4. Copy and fill [assets/templates](assets/templates). If the conversation already covered the task, record that first: requirements, findings, and each decision as accepted by the user, recommended by you, or open.
5. Ensure the memory pointer.

## Documents

Keep every claim traceable to the user, the code or a dated note; mark unverified interpretations as assumptions.

- `README.md`: request, scope, non-goals, **Current state** (the live design in a few bullets, not decision IDs), links to every doc.
- `AGENTS.md`: rules for this task only; link repo instructions rather than copy them. Suggest moving any repo-wide convention you find ("migrations are generated with `dotnet ef`") to the repo's CLAUDE.md or memory.
- `ARCHITECTURE.md`: current and target ownership, data flow and seams, from evidence.
- `DECISIONS.md`: accepted, recommended and open items with stable IDs; never promote a recommendation silently. Move superseded ones, marked with their replacement, to `## Superseded` at the bottom (or `DECISIONS-ARCHIVE.md` once long).
- `PLAN.md`: units and steps with status, commit hash and notes.
- `NOTES.md`: dated log of user direction, evidence, checks run and syncs.

Optional:

- Topic docs (catalogs, protocols) in the root, linked from README.
- `deck/`: decision-deck output.
- `inputs/`: files the user provides. `poc/`: proofs of concept.
- `briefs/<unit>-<step>.md`: one per subagent task, linking the relevant docs and decision IDs and giving absolute repo paths. The subagent prompt is only "Read and follow the brief at <absolute path>".
- `research/NN-topic.md`: one report per subagent, the only file it may write. Subagents don't edit the core docs; you do.

Never put workspace artifacts in `/tmp`.

## Resume

Read README → AGENTS → PLAN → the tail of NOTES → the relevant DECISIONS. Check `git status` and `git log` against PLAN's status rows and fix drift. Ensure the memory pointer.

## Sync

Keep the workspace current without being asked:

- When a decision is accepted or supersedes another, update in the same step every artifact that states the old position: ARCHITECTURE, PLAN, README's Current state, decks, and the Jira ticket if the workspace tracks one. Log the sync in NOTES.
- When you commit implementation work, set the PLAN row's status and commit hash in the same step.

## Other workspaces

Read them read-only when relevant. Modify one only with the user's OK, except to mark it superseded. Shared kits such as `deck-kit/` are exempt.

To split or supersede a workspace within the same arc: put "Superseded by `<new>` on <date>" at the top of the old README, link the old workspace from the new README, and point the memory entry at the new one.

## Memory

If your harness has persistent memory, keep one entry pointing to the workspace path. If an entry points to a workspace that no longer exists, tell the user; don't follow or delete it silently.
