---
title: "TSDeck: Ninite for Homelabbers"
date: "2026-10-31"
description: "Pick self-hosted apps, get a one-line curl command, and end up with Docker containers behind Tailscale HTTPS."
category: 'Tutorial'
writing: false
published: true
tags: ["homelab", "docker", "tailscale", "cloudflare"]
---

TSDeck is a catalog of self-hosted apps that writes a bash installer for the ones you pick. You choose apps in the browser. You get one `curl` command. You run it on a Linux server. Docker containers come up, and each app answers on your tailnet over HTTPS.

No account, no agent, no YAML to copy by hand.

## Browse the catalog

The site at [tsdeck.rishavnandi.workers.dev](https://tsdeck.rishavnandi.workers.dev) lists 133 apps. They are grouped by category: media management, documents, networking, files, monitoring, home, security, AI, and a dozen more.

There is a search box. Each entry shows a name, a description, and warnings where they matter. Jellyfin, for example, warns that hardware transcoding needs a GPU passed into the container.

Selecting an app that works better with another one does not block you. The API returns a warning instead, so you can install Sonarr without Prowlarr if that is what you want.

## Generate the command

Press **Generate Install Command**. The frontend sends the selected app IDs to `POST /generate`.

The Worker checks that every ID is real, applies a rate limit of ten requests a minute per IP, and writes the selection to Cloudflare KV under a random eight-character slug. The KV entry expires after 900 seconds, so the command is good for 15 minutes.

The response contains the slug, the full URL, and any warnings. The page prints:

```bash
curl -fsSL https://tsdeck.rishavnandi.workers.dev/s/<slug> | bash
```

That URL serves a plain bash script with `Content-Type: text/x-shellscript`. Nothing is stored about your server, because the Worker never talks to it.

## What the bootstrap does

Run the command as root, or as a user with sudo. The script starts with `set -euo pipefail`, so a failed step stops the run.

### Docker

It checks for `docker`. If the binary is missing, it installs Docker with `get.docker.com`, enables the service through systemd or sysvinit, and adds the current user to the `docker` group.

Then it waits for the daemon. It tries `docker info` as your user, falls back to `sudo docker info`, and gives up after ten attempts with a hint about `newgrp docker`.

### Prompts

Everything secret is typed on your machine, not sent anywhere.

First prompt: a Tailscale auth key, read with hidden input. Create one at `login.tailscale.com/admin/settings/keys`.

Then the script asks for each app's required environment variables. The generator collects these from every selected app and removes duplicates, so a shared `TZ` or `DATA_DIR` is asked once. A variable already exported in your shell is skipped. Any name containing `password`, `token`, `key`, or `secret` is read hidden. Pressing Enter accepts the default.

### Compose assembly

The script fetches `compose.yaml` for each app from `raw.githubusercontent.com/rishavnandi/tsdeck/main/apps/<app>/`.

It does not concatenate whole files. It walks each template and copies the `services:` block and the `volumes:` block, then writes them under one `services:` header in `/opt/tsdeck/docker-compose.yml`. Values go to `/opt/tsdeck/.env`.

It also writes `/opt/tsdeck/config/tsdproxy.yaml`, substituting your auth key into the Tailscale provider stanza.

### Start

The assembled file always includes a `tsdproxy` service: the `almeidapaulopt/tsdproxy` image, the Docker socket mounted, port 8080, and the config directory. TSDProxy reads the `tsdproxy.enable` and `tsdproxy.name` labels on each container and publishes it on the tailnet with a TLS certificate.

Finally it runs `docker compose up -d --remove-orphans --force-recreate`, with a sudo retry.

## Access and re-runs

Apps appear at `https://app-name.your-tailnet.ts.net`, using the name in the label. Immich lands on `immich`, Jellyfin on `jellyfin`.

Data lives in named volumes and bind mounts declared by the templates, not in the containers. Re-running a generated command recreates containers but keeps those volumes, so a config change or an added app does not wipe your library. `--remove-orphans` drops containers from apps you deselected, and their volumes stay on disk.

Set `DRY_RUN=1` to print every action without changing the system. Set `TSDECK_DIR` to install somewhere other than `/opt/tsdeck`. Read the script before piping it to bash; it is short and has no obfuscation.

## Architecture

- Static frontend, served by the same Worker as Cloudflare static assets.
- Hono API on Cloudflare Workers: `/apps`, `/generate`, `/s/:slug`.
- Cloudflare KV holds only the app ID list, with a 15-minute TTL. No keys, no passwords.
- Compose templates and the TSDProxy config live in the GitHub repo.

The trust boundary is simple: the webapp decides which templates to fetch, and your server decides everything else.

Code: [github.com/rishavnandi/tsdeck](https://github.com/rishavnandi/tsdeck). Live: [tsdeck.rishavnandi.workers.dev](https://tsdeck.rishavnandi.workers.dev).
