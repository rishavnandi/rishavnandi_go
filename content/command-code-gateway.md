---
title: "Pooling Cheap Command Code Accounts Behind One Router"
date: "2026-10-31"
description: "Three VPSes, two low-cost accounts, one bridge per host, and an OpenCode client that sees a single endpoint."
category: 'Writeup'
writing: false
published: true
tags: ["ansible", "tailscale", "opencode", "selfhosted"]
---

Command Code's Go plan costs a dollar, and its limits count against one account over a rolling window. Two accounts means two windows. The obvious move — put both keys on one box — defeats itself. Every request would leave from the same address, and source IP is the first thing anyone looks at when two cheap accounts behave like one user.

So each account needs its own egress. That one constraint decides the whole layout.

## The layout

Three VPSes on one Tailscale tailnet, all Ubuntu 24.04, reached over Tailscale SSH as `opc`.

`loki` is the router: aarch64, 23.4G RAM, 4 vCPU. Every request passes through it, and it holds the per-key bookkeeping.

`heimdall` and `thor` are the bridges: Oracle AMD boxes with 0.9G RAM and an 8G swapfile, each holding exactly one Go account key. One key per bridge, one bridge per host, one host per IP, so the two accounts never share an address.

The third box is the router, not a third bridge. Throughput would be the same either way, because quota is counted per account. If a third account arrives, the ARM box has room to host its bridge too.

## The router

`commandcode-router` ships in the same upstream repository as the bridge. It listens on `:9992`, takes a comma-separated list of `id=url` backends, and spreads requests across them. When a backend fails it cools that backend for a flat 60 seconds and moves on. The laptop never learns that more than one machine exists.

One secret does all the work. `shared_secret` in the vault becomes the credential OpenCode presents to the router, the credential the router presents to each bridge, and each bridge's own `BRIDGE_API_KEY`. One string, three places.

## Compose per host

Every box runs Docker Compose, and every container uses `network_mode: host` instead of published ports. That avoids rootless port publishing and lets `HOST=0.0.0.0` bind. The two bridge compose files stay identical, which is worth more than tidiness on the ARM box.

The router needs a different entrypoint, so it gets its own file and runs `node dist/router-index.js`.

## The Ansible side

One playbook drives the fleet. Per host it installs Docker CE with compose and buildx, clones the upstream bridge at a pinned release tag rather than `main`, renders an env file from Ansible Vault at mode `0600` with `no_log: true`, builds the image, starts the container under compose, and then asserts that the container is actually running.

A tag makes the deploy reproducible, and moving it back is the rollback.

The env template is the only per-host difference. It picks the key for the host it runs on:

```
HOST=0.0.0.0
BRIDGE_API_KEY={{ vault_data.shared_secret }}
COMMANDCODE_API_KEY={{ vault_data.cc_keys[inventory_hostname] }}
COMMANDCODE_CLI_VERSION={{ cc_cli_version }}
```

The playbook also settles a trap. The bridge bakes in CLI version `1.74.0`, and the published CLI is `1.84.0`. Ten releases of drift eventually produces a hard 403 that names a version and looks nothing like a rate limit. So each run reads `npm view command-code version` on the controller — the VPSes have no npm, only the container does — and writes the result into every env file. The pin refreshes when you run the playbook, not on a timer. No cron, no workflows, no scripts on the hosts, no registry.

The first run is the experiment. On one vCPU with an 8G swapfile, the upstream project's full verify suite may be slow or thrash. Run `--limit heimdall` first; the version pin sits inside the fleet play, so a limit does not skip it.

## The laptop

OpenCode runs on the laptop with one custom provider pointed at `http://100.98.210.60:9992/v1`. Nothing else changes: no per-account switching, no second config. The models block has to be written by hand, because OpenCode only auto-discovers models from Ollama, LM Studio and vLLM.

## What this costs

Failover sits one layer up. With one key per bridge there is nothing local to fail over to, so it all lands on the router, and the router ignores `Retry-After`. An account whose window is spent gets probed again every minute instead of being parked until it resets. Nothing breaks, since the other account still serves. It just chatters.

A single container holding both keys would park the spent account properly. It would also put both accounts behind one address, which is the one thing this design exists to avoid.

## The risk

This is a workaround for per-account limits. The bridge does not bypass those limits — each account still runs on its own quota — but pooling two accounts for more total throughput is a question for Command Code, not a configuration trick.

Running this may violate the provider's terms. Accounts can be suspended or banned, and that can happen without warning and without an error that explains itself. The layout keeps egress IPs and project slugs distinct. That does not hide anything from an operator who decides to look.

Do not use this to farm free capacity or to resell access. If one account no longer covers your work, a paid plan is the answer.

The playbook, the compose files and the full design notes are in [command_code_gateway](https://github.com/rishavnandi/command_code_gateway).
