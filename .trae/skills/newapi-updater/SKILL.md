---
name: newapi-updater
description: Safely synchronize this New API fork with the official upstream while preserving custom commits. Use when the user asks to update, sync, rebase, merge, or rebuild this repository. Do not discard custom work or force-reset branches.
---

# New API Updater

Use this workflow for requests to update the New API source or rebuild its custom image.

## Repository model

- Work only in `/Users/jairo/Desktop/newapi-custom` unless the user identifies another checkout.
- Keep `upstream` pointed at `https://github.com/QuantumNous/new-api.git`.
- Keep `origin` pointed at the user's fork.
- Keep `main` as the clean mirror of `upstream/main`.
- Keep custom work on branches named `custom/*`.

## Before changing anything

1. Run `git status --short --branch`, `git remote -v`, and `git branch -vv`.
2. Never overwrite uncommitted changes. If the worktree is dirty, summarize the files and ask whether to commit or stash them.
3. Confirm the current branch is not `main` before applying custom changes.
4. Do not expose GitHub tokens, environment secrets, or provider keys in output.

## Update workflow

1. Fetch official changes with `git fetch upstream`.
2. Update the clean branch with `git switch main` followed by `git merge --ff-only upstream/main`.
3. Return to the active `custom/*` branch.
4. Rebase the custom branch onto `main` with `git rebase main`.
5. If conflicts occur, stop and report the conflicted files. Do not guess resolutions, reset, or force-push.
6. Run the repository checks appropriate to the changed files.
7. Push the updated custom branch with `git push --force-with-lease origin <branch>` only after the user has approved the rewritten history.

## New custom changes

1. Create a new `custom/<short-description>` branch from the current `main`.
2. Keep each logical change in a separate commit.
3. Add focused regression tests for behavior changes.
4. Never commit secrets or generated local data.

## Image workflow

- Build from the active custom branch, not from `main`.
- Use `docker build --platform linux/arm64 -t calciumion/new-api:last -f Dockerfile .` on Apple Silicon.
- Use `linux/amd64` on Intel Macs.
- After a successful build, verify with `docker image inspect calciumion/new-api:last --format '{{.Id}} {{.Os}}/{{.Architecture}} {{.Created}}'`.
- Do not push an image unless the user explicitly requests it.

## Completion report

Report the active branch, synchronized upstream commit, custom commits preserved, conflicts or blockers, checks run, image tag if built, and whether any remote push occurred.
