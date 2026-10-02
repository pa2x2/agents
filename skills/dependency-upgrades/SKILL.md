---
name: dependency-upgrades
description: Audit a project's dependencies and write a report of available upgrades, grouped by risk, with the gain, the likely failures and a verification plan for each group. Then carry out the groups the user picks, one at a time, with verification strong enough to catch regressions before they ship. Use this whenever the user wants to upgrade, update or bump dependencies, packages, libraries, plugins, SDKs or toolchains, asks what is outdated or can be upgraded, asks whether an update is safe or worth it, or pastes a list or table of upgrades and asks to perform them "without regressions".
---

# Dependency upgrades

The work has two phases.

1. **Report.** Read-only. Find what can be upgraded, what each upgrade gives this project, what can go wrong here specifically, and how to verify it. Group the upgrades by risk and recommend an order. Then stop.
2. **Upgrade.** Only for the groups the user picks. Apply, fix, verify, report.

Grouping by risk matters for two reasons. Each group becomes one commit and one verification pass, so a regression points to one cause. And the user can stop before a group whose risk they don't want to take on right now.

## Phase 1: report

### 1. Find everything that pins a version

Versions hide in more places than the main manifest:

- Manifests, lockfiles, version catalogs, BOMs and platform constraints.
- Toolchain pins: Gradle wrapper, `packageManager` field, `.nvmrc`, `.tool-versions`, `rust-toolchain.toml`, `requires-python`, `go` directive.
- Platform levels: compileSdk, targetSdk, minSdk, NDK, iOS deployment target, framework SDK (Expo SDK, Flutter SDK).
- Dependencies pinned to a git commit (JitPack, git URLs, forks) and prebuilt binaries checked into the repo, along with the scripts that build them.
- Shared version knobs. A version that another script also reads (an NDK version that an AAR build script uses) can't be bumped on its own.

Find out what runtime targets exist for testing: connected devices, emulators or simulators (`adb devices`, `emulator -list-avds`, `xcrun simctl list`), browsers, a local server. Some checks in the plan will need them.

### 2. Find what is available

Ask the registries. Don't rely on memory.

Sort every candidate into one of these:

- Newer stable release.
- Prerelease only (alpha, beta, RC, preview).
- Blocked by a constraint: a framework SDK that pins a set of versions, a peer dependency range, a plugin's supported tool versions. Name the constraint.
- Commit-pinned. Compare against the upstream head, and check that the pinned artifact is still served (see failure mode "Artifact only resolves from cache").

If the project deliberately uses an alpha line (say a Compose alpha BOM), compare within that line. The newest "stable" may be a downgrade.

For BOMs and platform bundles, find out what a bump actually moves. A BOM bump that only moves one artifact is a one-artifact upgrade.

### 3. Read what changed and match it to this codebase

For each candidate, read the release notes for every version between current and target, not only the target. Then search the codebase for each changed API, behavior, default, keep rule or config option.

This is what makes the report worth reading. "May contain breaking changes" tells the user nothing. This does: "`Slider` now handles `onValueChange` itself instead of passing it to `SliderState`. `presentation-core/.../Slider.kt:37-50` creates a `SliderState` and also passes `onValueChange`, which is exactly the pattern that changed. Dragging could jump or stop updating."

Give `file:line` references for every match. Mark guesses as guesses. If you couldn't read a changelog (rate limit, no changelog, native code), say so and rate the risk higher.

### 4. State the gain

For each upgrade, say what this project gets from it: fixes for bugs in code paths it uses, security fixes, performance, a feature it needs, or unblocking another upgrade. "Nothing" is a valid answer. A new Firebase BoM that changes none of the artifacts the project uses has no gain.

An upgrade with no gain and a real cost (new warnings you can't fix, a migration, a native rebuild) goes under "held back", with the condition that would change that ("revisit with the next ONNX Runtime rebuild").

### 5. Rate the risk

Semver doesn't measure risk. Rate each upgrade on three things:

- **Likelihood.** Do the changes touch APIs, behavior or config that this code uses?
- **Blast radius.** What breaks if it goes wrong? The build, one screen, background work, stored user data, authentication, every launch?
- **Detectability.** Where would the failure show up? At compile time (the good outcome), in tests, only at runtime, only in a minified release build, only when upgrading over existing data, or silently (a connection that hangs, a job that never runs)? Low detectability raises risk more than anything else.

Then place it in a tier:

| Tier | Typical content | Commit |
|---|---|---|
| Very low | Patch or small minor. Changelog read, nothing changed that this code uses, any failure would show at build time. | Several can share one commit. |
| Low | Small behavior changes in areas used here, native code whose changelog you couldn't fully read, tooling with cache or config changes. | One commit per group, a quick runtime check. |
| Medium | Generated code changes, storage or migrations, UI component behavior on call sites you found, binary compatibility between separately pinned artifacts, lifecycle or timing changes, security-sensitive flows. | One commit per dependency. |
| High | Platform target changes (targetSdk, runtime or framework majors), anything that needs new permissions, user-facing decisions or a migration plan. | Its own project with its own plan. Not a bump. |
| Held back | Prerelease only, blocked by a constraint, or no gain. | None. Say what unblocks it. |

### 6. Order the groups

- Build tooling first, so library changes get tested on the toolchain you'll ship. Exception: risky tooling with little gain can wait until the end, or be deferred.
- Cheap, safe groups before risky ones.
- Upgrades that must move together go together: framework SDK sets, BOM members, a library that needs a newer compileSdk. Note such couplings explicitly, since they can force a group to grow.
- Anything that could cause a regression on its own gets its own commit.

### 7. Write the report

Use this structure. Each group has to stand on its own, because the user may paste one group into a new session. Repeat the file locations, versions and checks in every group rather than writing "as above".

```markdown
# Dependency upgrade plan: <project>

## Recommended order

| # | Group | Risk | Main check |
|---|---|---|---|

## Group <n>: <name> (<tier> risk)

Declared in: <files>

| Dependency | Current → target | Gain for this project | What could go wrong here |
|---|---|---|---|

<For medium and high risk items, a short section per dependency: the specific changes, the matching code with file:line, and why the failure would or wouldn't be caught at build time.>

Verification:
- Gate: <the exact commands from the repo rules and CI>
- Build: <variants to build, including the release/minified one when relevant>
- Runtime: <each affected flow as start state → action → expected end state>
- Data: <upgrade over existing data, migration from the last released version, if relevant>
- Needs: <devices, emulators, accounts or data you don't have>

## Held back
- <dependency> <current → available>: <reason>. Revisit when <condition>.

## Not verified
<Changelogs you couldn't read, guesses you couldn't confirm.>
```

## Phase 2: upgrade

Work through the groups the user authorized, in order. Keep each group's changes in its own commit and keep unrelated fixes out of it. Commit only if the user has authorized commits for this work; otherwise leave the changes uncommitted and say so.

### Before changing anything

1. Check the worktree. If it has changes you didn't make, leave them out of everything you stage.
2. Run the project's verification gate once on the unchanged tree and note the warnings and test counts. Later, a warning only counts as "caused by the upgrade" if it wasn't there before. When you can't tell, build the base commit in a separate git worktree and compare.
3. For dependencies that generate code (SQL, protobuf, GraphQL, ORMs, serializers), save the generated output so you can diff it after the upgrade.
4. If a library reads or writes a format the project persists (editor markdown, serialized settings, a database), write a characterization test that passes on the current version first. If it still passes after the upgrade, the format survived. If the project has no test for that contract, this test is worth keeping.

### Apply

- Use exactly the versions in the plan or the user's message. If something newer exists, mention it. Don't quietly go further.
- Prefer the ecosystem's own tool (`./gradlew wrapper`, `npx expo install --fix`, `cargo update -p`). Then read the whole diff. Generators reset custom settings; the Gradle wrapper task, for example, dropped a deliberate `retries=3`. Restore such settings in a way that survives the next regeneration.
- Inspect the resolved dependency graph afterwards. Every target version should resolve as intended, nothing else should have moved unexpectedly, and nothing the code imports directly should have disappeared from the graph.

### Fix what breaks

- Fix the cause. Don't suppress warnings, disable lint rules, add baseline entries, add catch-all branches or pin around a problem to get a green build. If a stricter lint rule or a new compiler check flags something real, improve the code. The user has said explicitly: no cheap fixes.
- If the code used something it only got transitively, declare it explicitly. If that thing is deprecated, say so; replacing it is separate work.
- If the proper fix is a large redesign unrelated to the upgrade, stop and ask. Offer the choices: do it now, a narrow interim fix, or defer the upgrade.
- If an upgrade turns out to give nothing and costs something (an upstream deprecation warning on every build that you can't fix locally), revert it and report it as deferred, with what would change that.

### Verify

Run every level the group's plan calls for, and extend the plan when the changes you made touch more than expected. Report evidence for each level, not just "passed".

1. **Static.** Formatting, lint, typecheck and compilation of every variant, flavor and target the project ships.
2. **Tests.** The full suite CI runs, plus focused tests for code you touched.
3. **Dependency audit.** Resolved versions match. For git-pinned or unusually hosted artifacts, resolve once without caches; a build that only works from cache will fail on the next clean machine.
4. **Production-equivalent build.** Minified, optimized, tree-shaken or AOT. Many upgrade regressions only exist there.
5. **Runtime, in place.** Install as an update over existing data, not a fresh install, and confirm the new build is the one running (new process, new version). This is where migrations, cached state and stored formats break.
6. **Affected features, end to end.** Exercise each affected feature through complete state transitions, not "the screen opens". Enable a setting, let it persist, cold-restart, re-enter the flow, complete it, then restore the setting. Repeat cycles where timing matters. Include what lives outside the compile graph: plugins or extensions loaded at runtime, background and scheduled jobs, widgets, deep links. Watch the logs for crashes and linkage errors during all of it.
7. **Release runtime.** Install the production-like build and repeat the critical paths. In one past upgrade this step alone found a class that the minifier removed but a runtime-loaded extension still needed.

When a check can't run (no device with the right OS version, no test account), say which risk stays unverified instead of implying coverage.

### Report each group

- Commit and changed files, or "uncommitted".
- Code changes the upgrade needed, and why.
- Verification: what you actually exercised and what you observed.
- Problems found that the upgrade didn't cause (confirmed against the base commit). Report them separately and ask before fixing.
- What you deferred, and why.
- What you couldn't verify.
- What you left on the machine or device.

Then go on to the next authorized group, or stop and wait.
