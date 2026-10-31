---
title: "Unlocking an Airtel ZTE F670L ONT"
date: "2026-10-31"
description: "How I opened telnet, cleared the region lock, restored PPPoE and SIP, and reached bridge mode on an Airtel Xstream ZTE F670L."
category: 'Tutorial'
writing: false
published: true
tags: ["networking", "zte", "ont", "airtel", "homelab"]
---

## The region lock is one number

The ONT stores a region code in its config. Airtel India provisions **35**. The stock manufacturer code is **198**. Every locked behaviour follows from that number: you cannot edit the WAN profile freely, and the bridge-mode option is hidden or absent.

Changing the code back to 198 is the whole unlock. The rest is getting a shell and putting your connection back together.

This is ISP equipment. Unlocking it voids your warranty and support with Airtel, can break the ONT, and may violate the ISP's terms of service. Do it only on hardware you are allowed to modify.

## Back up before you touch anything

`upgradetest sdefconf 198` resets every setting to factory defaults. The connection dies and you re-enter everything by hand.

Screenshot these first:

- **Network -> WAN**, which shows the PPPoE username, the VLAN ID, and the 802.1p priority
- **Voice -> VoIP**, the SIP account
- Any WiFi SSID and password you care about
- The firmware version from the status page

Settle the PPPoE password while the line still works. The username looks like `0XXXXXXXXXXX_dsl@airtelbroadband.in`. The password is usually your Airtel account number from the Thanks app. On lines converted from DSL to fiber it is the old Relationship ID from an Xstream eBill instead.

## Opening telnet

Telnet is disabled on this firmware. The factory-mode handshake opens it and returns temporary credentials:

```sh
uv run zte_unlock -i 192.168.1.1
```

`uv run` creates the virtualenv and installs the two dependencies, `cryptography` and `psutil`. The tool prints a telnet username and password when it succeeds.

The interface must carry the magic MAC `00:07:29:55:35:57`. Windows will not set a `00:`-prefixed MAC on a wireless adapter, so use Ethernet. To check the interface before you start:

```sh
uv run zte_unlock -i 192.168.1.1 --check-mac
```

If every attempt fails, pass your real web UI credentials. Airtel locks the built-in factory account about a month after activation:

```sh
uv run zte_unlock -i 192.168.1.1 -u <webui_user> -w <webui_password>
```

Then log in with `telnet 192.168.1.1`. Credentials rotate on every reboot, so a login that worked yesterday can fail today. Re-run the tool.

## Clearing the lock

From the telnet shell:

```sh
upgradetest sdefconf 198
```

Use the bare command. The `setmac 4 198/upgradetest sdefconf 198` variant quoted in the source thread does not work. Dropping the `setmac` part is what made it succeed.

To undo it:

```sh
upgradetest sdefconf 35
```

Setting the code back to 35 lets TR-069 push the Airtel profile again. That also restores PPPoE and SIP if you lost them, so nothing here is a one-way door.

## Restoring PPPoE

The connection is dead after the unlock. Open **Network -> WAN** and rebuild it as PPPoE with the username and password from your screenshot.

Get the VLAN ID right. A wrong VLAN looks exactly like a wrong password. Leave 802.1p as it was.

Uncheck "Default gateway" if a downstream router will own the gateway. Leave it checked if the ONT routes for you.

## Restoring SIP

Under **Voice -> VoIP**, the account is your landline number in E.164 form, `+91` and no leading zero. The authorization username is that number followed by `@<state>.ims.airtel.in`. The password is the one from your screenshot.

Bihar and Jharkhand both use `bh` as the state code. Other circles use their own. If the values do not authenticate, recover them from the ISP profile instead of guessing.

## Bridge mode, optional

First check **System Management -> Software Upgrade** exists. If Airtel removed that page, this section cannot work for you.

The bridge option is present in the WAN page but hidden, and on current firmware the setting does not survive a reboot. The fix is a downgrade to `V9.0.11P1N40`, a 2022 build, even though the page calls it an upgrade. Verify the image hash, upload it, and let the ONT reboot.

Then open **Network -> WAN**, right-click the connection-type dropdown, choose Inspect, and delete the `hidden` attribute on the `bridge` option. Select Bridge and save.

Bridge mode alone passes no traffic. Bind it under **Network -> Port Binding** to a LAN port, set that port's DHCP Port Control to Internet, and match the VLAN and 802.1p of the routed WAN profile. Untick the same port on the routed profile, since a port in two profiles is undefined behaviour.

Run Ethernet from that LAN port to your router's WAN port and dial PPPoE there. Leave VLAN tagging off on the router. Turn the ONT's WiFi off while it bridges, or you get two DHCP servers on one segment.

## Firmware compatibility

| Firmware | Telnet | `sdefconf 198` | Bridge trick |
| --- | --- | --- | --- |
| `V9.0.10P18N2B` | works | works | no, reverts on reboot |
| `V9.0.11P1N40` | n/a | n/a | works, persists |
| `V9.0.10P18N3D` | broken | n/a | broken |

## When it fails

The one error worth memorising is this, repeated across all five retries:

```
step [2] send sq: unknown error
```

Steps 0 and 1 succeeding means the tool reached the ONT. Step 2 is the payload, and the firmware rejected it. That is a patch in the HTTP handler, not a header or MAC problem. On `18N3D` and later, neither telnet nor the bridge trick works, and there is no workaround in the repo.

## Source

The procedure and the command came from `@BornArcher8` and the [india broadband forum](https://broadband.forum/) threads on F670L telnet access, bridge mode, and SIP setup. The SIP values and the downgrade path were my own trial and error.

Code and the full walkthrough: [rishavnandi/zte_f670l_unlock](https://github.com/rishavnandi/zte_f670l_unlock).
