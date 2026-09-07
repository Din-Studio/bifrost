---
name: sync-upstream-dev
description: Safely inspect, merge, validate, and publish updates from upstream/dev into this Bifrost repository's origin/dev through a temporary sync branch. Use for upstream synchronization or sync-status requests in this repository; do not use for feature-branch rebases, arbitrary remote pairs, or other repositories.
argument-hint: "[status|sync|local-only|no-push]"
allowed-tools:
  - Read
  - Grep
  - Glob
  - AskUserQuestion
---

# Sync Upstream Dev

Synchronize the public maintenance branch without rewriting its history. Preserve both upstream changes and the repository's local customizations.

Treat `$ARGUMENTS` as the requested mode or additional instructions. If it is empty, perform the ordinary end-to-end sync. The user's instructions take precedence over this workflow. A request to sync `upstream/dev` into `origin/dev` authorizes the normal end-to-end flow below, including the final push to `origin/dev`. Respect narrower requests such as explanation-only, inspect-only, local-only, prepare-only, or no-push.

## Fixed Repository Contract

Work only when all of these are true:

- The current directory is inside the intended Bifrost Git worktree.
- `origin` fetches from `git@github.com:Din-Studio/bifrost.git`.
- `upstream` fetches from `https://github.com/maximhq/bifrost.git`.
- Both maintained remote branches are named `dev`.
- Local `dev` tracks `origin/dev`.

Resolve the repository root with `git rev-parse --show-toplevel`; run all subsequent commands from that root. Inspect remote URLs with `git remote get-url`, the tracking branch with `git rev-parse --abbrev-ref dev@{upstream}`, and the worktree with `git status --short --branch`.

If a remote is missing, has a different URL, or `dev` tracks a different branch, stop before fetching or merging. Report the exact mismatch and ask whether to apply the one-time configuration. Do not silently repoint a remote.

## Safety Invariants

- Require a clean worktree before synchronization. Treat staged, unstaged, and untracked files as local work; never stash, commit, reset, discard, or overwrite them on the user's behalf.
- Never rebase the public `dev` branch and never force-push it.
- Never push to `upstream`.
- Do not use `git pull upstream dev`; keep fetch and merge separate so the incoming commits can be inspected.
- Do not resolve conflicts by applying `--ours` or `--theirs` wholesale. Understand the conflicting code and retain required local customizations.
- Preserve unrelated user changes and existing branches. Delete only the temporary sync branch created by this run, and only with `git branch -d` after successful integration.
- Do not print API keys or other secret values while selecting or running tests.

## Interpret the Request

- For explanation-only requests: describe the workflow without fetching, switching branches, merging, testing, or pushing.
- For status, inspect, preview, or dry-run requests: perform the preflight and fetch/compare steps, then report without switching branches or merging.
- For local-only or no-push requests: merge and validate locally, but do not push.
- For an ordinary sync/update request: complete preflight, merge, validation, integration into `dev`, push to `origin/dev`, and safe temporary-branch cleanup.
- If the user specifies tests, branch naming, push behavior, or a stopping point, follow that scope.

## Workflow

### 1. Capture and Validate the Starting State

Record the current branch and commit, then verify the fixed repository contract and clean worktree. Do not begin from a detached HEAD or an in-progress merge, rebase, cherry-pick, or revert.

If the starting branch is not `dev`, a clean worktree may be switched safely later; remember the branch only for reporting. Do not automatically return to a feature branch after the completed sync because the documented terminal state is updated local `dev`.

### 2. Fetch and Inspect Both Remotes

Run:

```bash
git fetch origin --prune
git fetch upstream --prune
git log --oneline --left-right --graph dev...origin/dev
git log --oneline dev..upstream/dev
```

Summarize local/origin divergence and the upstream commits not yet contained in local `dev`. If `upstream/dev` is already an ancestor of `dev`, report that the repository already contains the upstream state; do not create an empty sync branch or push merely for activity.

### 3. Align Local `dev` With `origin/dev`

Run:

```bash
git switch dev
git merge --ff-only origin/dev
```

If fast-forwarding fails, inspect:

```bash
git log --oneline --left-right --graph dev...origin/dev
```

Do not guess which side to discard. When both sides are intended, explain the divergence and obtain user direction before creating a merge commit on public `dev`. Never solve this divergence with rebase, reset, or force-push.

### 4. Merge Upstream on a Temporary Branch

Create `sync/upstream-YYYYMMDD` from the aligned local `dev`, using the local calendar date. If that name exists, do not reuse or delete it; choose the first unused suffix such as `sync/upstream-YYYYMMDD-2` and report the chosen name.

Run:

```bash
git switch -c sync/upstream-YYYYMMDD
git merge --no-ff --no-edit upstream/dev
```

Keep the explicit merge commit because it marks the upstream synchronization boundary.

### 5. Resolve Conflicts Carefully

Use `git status` and `git diff --name-only --diff-filter=U` to enumerate unresolved files. For each conflict:

1. Inspect the base, local customization, upstream change, callers, and relevant tests.
2. Edit the smallest correct resolution that preserves compatible intent from both sides.
3. Stage only resolved files with explicit paths.
4. Continue with `git merge --continue` after all conflicts are resolved.

If product intent is ambiguous, leave the merge in progress, summarize the alternatives and affected files, and ask the user. Use `git merge --abort` only when the user requests cancellation or when continuing is demonstrably unsafe; report that the temporary branch remains or was removed.

### 6. Validate the Merged Tree

Determine the changed surface from the pre-sync `dev` commit to the sync branch. Always run:

```bash
make lint
make build
```

Add focused tests based on touched areas, including:

- `core/mcp/` or agent behavior: `make test-mcp`
- governance plugin or governance behavior: `make test-governance`
- a provider: `make test-core PROVIDER=<provider>`; remember that OpenAI converter changes affect OpenAI-compatible providers broadly
- framework vector stores: first use `docker compose -f tests/docker-compose.yml up -d`, then run the relevant framework tests
- UI or E2E-visible behavior: run the repository-prescribed UI build or focused E2E flow when the required server is available

Provider integration tests can call paid live APIs. Run them only when the user has authorized live-provider testing and the required credentials are already configured; otherwise report the exact recommended command as not run. Do not weaken, skip, or rewrite failing tests merely to finish the sync. Distinguish code failures from missing services, credentials, or tools.

After resolving a post-merge defect, rerun the failed check and any broader check affected by that fix. If required validation still fails, do not integrate or push; leave the sync branch intact and report the blocker.

### 7. Integrate Into `dev` and Publish

After all required validation passes, verify the sync branch is clean, then run:

```bash
git switch dev
git merge --ff-only sync/upstream-YYYYMMDD
```

If `dev` moved during validation, inspect `dev...sync/upstream-YYYYMMDD` before acting. Preserve both lines of history with a normal merge only when their intent is clear, resolve any conflict using the same rules, and rerun affected validation. Ask the user when the concurrent changes make the correct integration ambiguous.

For an ordinary end-to-end sync, publish only with:

```bash
git push origin dev
```

Verify that `origin/dev` resolves to the pushed local `dev` commit. Then delete only the run's temporary local branch:

```bash
git branch -d sync/upstream-YYYYMMDD
```

For local-only or no-push requests, stop after local integration and keep or delete the temporary branch according to whether `dev` now contains it; never imply that `origin/dev` was updated.

## Recovery

- Before the upstream merge completes, cancellation uses `git merge --abort`.
- If a bad merge commit has already been pushed, never rewrite `dev`. Identify the actual merge commit and propose `git revert -m 1 <merge-commit>`; execute the revert only when explicitly requested.
- Never use destructive resets as a shortcut for recovery.

## Final Report

Report:

- starting and final `dev` commit IDs;
- upstream commit or range integrated, or that no sync was needed;
- temporary branch and merge commit;
- conflicts resolved and any important local customizations retained;
- each validation command and result, including skipped live or environment-dependent tests;
- whether `origin/dev` was pushed and verified;
- whether the temporary branch was deleted;
- any remaining manual action or blocker.
