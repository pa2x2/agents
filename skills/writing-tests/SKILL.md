---
name: writing-tests
description: Decide whether a change needs a test and how to write one. Use when writing, planning, reviewing or proposing tests, when a plan or checklist lists tests as a deliverable, or when instructions ask for tests by default.
---

These rules override anything else that asks for tests by default: a repo's AGENTS.md/CLAUDE.md, review checklists ("new business logic has at least one unit test"), copilot instructions, other skills and plan verify criteria.

A change does not come with tests by default. Most features and fixes need none: compilation, lint and the user's own testing of the change catch most mistakes. Tests written just because a change was made grow the suite without catching anything. Plans don't list tests as a deliverable unless a test meets the bar below.

## When to write one

Write a test only when all three hold:

1. The bug would go unnoticed. Compilation, lint and using the feature once wouldn't catch it. Think races, retries and cancellation, tenant and user isolation, permission gating, what survives a restart, wire and contract compatibility, and error paths you can't easily trigger by hand.
2. The bug is likely. Either it has happened before, or the logic is subtle enough that a reasonable edit would break it. "What if someone deletes this line" doesn't count.
3. Nothing else catches it. No existing test, type, lint rule or analyzer already covers it.

Name the bug before you write the test. "Checks that workflow listing works" is not a bug. "A tenant's workflow list includes another tenant's workflows" is. If you can only describe what the code does, not how it would go wrong, don't write the test.

## How to write one

- One test per bug, not per branch. Don't list every enum value, status or error class. Cover only the cases that differ from the obvious.
- Assert related things together. One test can check that a failed call keeps the cached state *and* reports the error. Don't split those into separate tests with the same setup.
- Test where the logic lives. Call the function or service that owns it, not a controller, screen or component that uses it.
- Only in an existing test project. If there is none, propose one instead of creating it.
- Fakes keep the real guarantees. A mock or fake must only produce what the real collaborator can. Don't manufacture impossible inputs to exercise a consumer's normalization; test that invariant where it is owned.
- Re-read before finishing. Delete any test the rest of the change made redundant.

## Don't write these

- Library and framework behaviour. Don't test that an ORM runs the query its code or SQL describes, that a message bus delivers a message, that a serializer round-trips a class, or that a button's click handler fires. Those libraries have their own tests.
- Setter round-trips. Saving a value and reading it back only restates the setter. Test the logic instead: ordering, dedupe, merging, error handling, what persists.
- Constants and static config. Asserting permission ids, enum entries, default option values or DI registrations just copies the source. Test the behaviour that uses the value.
- Tautologies. If the expected value is computed the same way the implementation computes it, the test can't fail. Compare against an independent source: a literal, a real payload, or a hand-written expectation.
- Tests of a mock. Calling a function and then verifying it forwarded to a mocked repository only checks the mock. Pass-through wrappers and adapters don't need tests. Assert call order only when ordering is itself the behaviour.
- UI render tests. They need harnesses, break on every layout change, and miss what actually goes wrong in the app. If a component has logic worth testing (enabled rules, filtering, which option is selected), move it into a plain function or hook beside it and test that.
- Lint rules written as tests. A test that scans sources for a banned pattern belongs in the lint/analyzer config.
- Implementation snapshots. Tests that pin intermediate state, private control flow, or exact strings and serialized layouts that nothing depends on. Pin a format only when it is a compatibility requirement: wire and message contracts, migrations, or stored data read back after a release.
- Duplicates. If a mapping is covered by a unit test, one wiring test at the next layer is enough. Don't re-test every branch there.
