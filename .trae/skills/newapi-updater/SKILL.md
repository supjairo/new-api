---
name: newapi-updater
description: Safely update this New API fork, build one concrete image, and recreate the configured container stack. Use when the user asks to update, deploy, rebuild, or synchronize New API. Never use symlinks or leave duplicate application images.
---

# New API Updater

Use this workflow for New API source updates, image builds, and remote deployments.

## Repository workflow

- Work in `/Users/jairo/Desktop/newapi-custom`.
- Keep `upstream` pointed at `https://github.com/QuantumNous/new-api.git`.
- Keep `origin` pointed at the user's fork.
- Keep `main` equal to the selected official upstream baseline.
- Keep custom work on `custom/*` branches.
- Before changing anything, run `git status --short --branch`, `git remote -v`, and `git branch -vv`.
- Never overwrite uncommitted changes. Stop and ask before stashing, resetting, or discarding them.
- Do not expose passwords, tokens, keys, or environment values in output.

## Update workflow

1. Fetch the requested official tag or branch from `upstream`.
2. Reset only the clean local `main` branch to that exact official baseline after confirming it has no uncommitted changes.
3. Create or rebase the active `custom/*` branch onto that baseline.
4. Preserve only intentional custom commits. Stop on conflicts and report the files instead of guessing.
5. Run backend tests and relevant checks before building.

## Image workflow

- Build one concrete application image from the active custom branch.
- Do not create or use symbolic links for image files, deployment directories, or compose files.
- Use a temporary archive only for transport, then delete it locally and remotely after loading.
- Use `docker build --platform linux/amd64 -t calciumion/new-api:last -f Dockerfile .` for x86_64 servers.
- Use `docker build --platform linux/arm64 -t calciumion/new-api:last -f Dockerfile .` for ARM64 servers.
- Keep only one application image tag on the target server: `calciumion/new-api:last`.
- Before loading the new image, remove obsolete application image tags and dangling layers after preserving the currently running container for rollback inspection.
- Never remove MySQL, Redis, persistent data, or persistent logs.

## Remote deployment workflow

1. Inspect the target architecture, compose project directory, service name, mounts, and current image.
2. Back up the real `docker-compose.yml` as a timestamped regular file, never as a symlink.
3. Load the new image and tag it only as `calciumion/new-api:last`.
4. Update the compose file to use exactly `calciumion/new-api:last`.
5. Run `docker compose up -d --force-recreate --no-deps new-api` or the target application service so the compose deployment uses the new concrete image.
6. Run `docker compose ps`, wait for the application health check, and request `/api/status` locally on the server.
7. Remove the temporary archive and prune only unused application image tags and dangling layers. Never run broad volume cleanup.
8. Verify that exactly one `calciumion/new-api` application image remains, the application container is healthy, and MySQL/Redis remain running.

## Completion report

Report the official baseline, custom branch and commit, concrete image ID and architecture, compose file backup path, recreated service, health status, `/api/status` result, cleanup result, and any blockers. State explicitly that no symlinks were used.
