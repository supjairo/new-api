---
name: newapi-fork-sync
description: Sync the New API fork with official upstream and rebase custom branches onto the latest baseline. Use when the user asks to pull official updates, merge upstream, sync the fork, or manage custom branches. Do not use for deployment or feature coding.
---

# New API Fork Sync

Two-track workflow: `main` mirrors the official repository, `custom/*` branches hold all user modifications rebased on top of it.

## Repository layout

- Work in `/Users/jairo/Desktop/newapi-custom`.
- `upstream` → `https://github.com/QuantumNous/new-api.git` (official, read-only).
- `origin` → `https://github.com/supjairo/new-api.git` (user's fork).
- `main` is an exact mirror of `upstream/main`: update only by fast-forward, never commit to it.
- `custom/*` branches contain all custom work, always rebased onto the tip of `main`.
- Before changing anything, run `git status --short --branch`, `git remote -v`, and `git branch -vv`.

## First-time setup

Run only when the remotes are missing or wrong:

1. If `origin` points at QuantumNous, run `git remote rename origin upstream`.
2. Add the fork: `git remote add origin https://github.com/supjairo/new-api.git`.
3. Push the official mirror: `git push origin main`.

## Safety rules

- Never overwrite uncommitted changes. Stop and ask before stashing, resetting, or discarding them.
- Never commit to `main`, never rewrite or force-push `main`.
- Push custom branches only with `--force-with-lease`, never bare `--force`.
- On rebase conflict, resolve only when the correct resolution is clear: keep the official upstream change, then reapply the custom intent on top. Otherwise run `git rebase --abort` and report the conflicted files instead of guessing.
- Do not expose passwords, tokens, keys, or environment values in output.

## Daily custom work

1. Enter the custom branch, or create it from the official baseline: `git switch custom/<name>` or `git switch -c custom/<name> main`.
2. Make changes and commit them on the custom branch.
3. Push: `git push origin custom/<name>` (first push of a new branch: `git push -u origin custom/<name>`).

## Sync workflow (official updated)

1. Fetch official history: `git fetch upstream`.
2. Fast-forward the official track:
   `git switch main && git merge --ff-only upstream/main`.
   If `main` cannot fast-forward, stop and report — never hard-reset it while unsure.
3. Mirror it to the fork: `git push origin main`.
4. For each custom branch to update (ask which ones when several exist):
   `git switch custom/<name> && git rebase main`.
5. Handle conflicts per the safety rules: fix files, `git add <files>`, `git rebase --continue`; abort and report when uncertain.
6. Push the rebased branch: `git push --force-with-lease origin custom/<name>`.

## Verification before reporting done

- `git log --oneline upstream/main..HEAD` must list only intentional custom commits.
- `git diff main...HEAD --stat` must match the expected custom footprint.
- Backend must build: `go build ./...`, and `cd relaykit && GOWORK=off go build ./...`.
- If `web/` changed: build the frontend (`bun install` in `web/` when `node_modules` is missing, then `bun run build`).
- Run focused tests for rebased commits that touch tested code.

## Completion report

Report the previous and new official baselines (old and new `upstream/main` commit), custom branches rebased with their commit counts, conflicts encountered and how they were resolved, push results for `main` and each custom branch, and build or test outcomes. State any remaining blockers explicitly.
