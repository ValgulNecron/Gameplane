# Packet Captures Needed

This file lists the 8 games whose join protocols lack usable public documentation and block the e2e test suite. A real client-to-server capture from each is the unblocking step.

## Why captures matter

The project has query-depth probes for 14 games (via Steam A2S and custom queries), but join-level automation exists only for 2 (Minecraft and Terraria, both with published protocol specs). The remaining 14 split into two groups:

1. **Partial documentation + iterative reverse-engineering** (6 games: Rust, Garry's Mod, CS2, Ark: Survival Ascended, Palworld, Satisfactory) — known enough to attempt implementation; blocked by incomplete public specs, not absence of all data.
2. **Zero documentation** (8 games, this file) — no wire-level handshake spec exists. A real capture is needed to establish the packet format; once captured, we decode it, implement a headless client, and iterate on a live test server until join succeeds.

## Factorio

**Port:** UDP 34197 (server-to-client listen)  
**Known:** Mutual random-ID exchange (FFF #149, #147). No further byte layout published.  
**Blocking:** A live server returns **absolutely nothing** on an unrecognized datagram. No query surface to probe blindly. Third-party reverse-engineer (`radioegor146/factorio-reverse`) exists but is incomplete.  
**Critical constraint:** **Version-locked.** Factorio server and client must match exactly. The capture must record the precise versions of both.

**Capture guidance:**  
Record the join handshake: client sends initial datagram, server replies, exchange continues until server accepts or rejects. ~10–20 packets total should suffice. Record both directions. Note exact client version (launcher or headless binary version string) and server version (visible in server logs or GUI).

---

## 7 Days to Die

**Port:** UDP 26900  
**Known:** Steam A2S query answers on port 26901 (our current depth).  
**Blocking:** No public protocol analysis. Modding ecosystem covers gameplay, never networking. Transport library unidentified; LiteNetLib was investigated and found no supporting evidence.

**Capture guidance:**  
Fresh join from main menu. 20–30 packets into the handshake should show the format. Exact client version (from launcher) and server version (from server console).

---

## V Rising

**Port:** UDP 9876 (query on 9877 answers A2S)  
**Known:** Unity engine + BepInEx modding ecosystem.  
**Blocking:** 36 public community repos exist; all are server managers or mods. None document the join protocol. Transport unidentified.

**Capture guidance:**  
Join attempt. 15–25 packets. Client + server versions (launcher and server).

---

## Project Zomboid

**Port:** UDP 16261 (A2S also answers on 16261)  
**Known:** Custom UDP. Supports non-Steam private servers, so join should be possible without Steam identity once the wire format is known.  
**Blocking:** Purely documentation — the format is unknown, not technically infeasible.

**Capture guidance:**  
Join to a non-Steam (private) server if possible, to isolate game protocol from Steam integration. 20–30 packets. Client + server versions.

---

## Don't Starve Together

**Port:** UDP 10999 (main world; caves shard on 11000 if relevant)  
**Known:** Klei's own reliable-UDP layer. Server boots offline without Klei authentication token, so a join should be possible without external credential services.  
**Blocking:** Reliable-UDP frame format and handshake are not documented.

**Capture guidance:**  
Join to a private/offline world. 20–30 packets. Client + server versions.

---

## Enshrouded

**Port:** UDP 15636 (query on 15637 answers A2S)  
**Known:** No in-game console by design. No anti-cheat gate or credential requirement identified.  
**Blocking:** Wire format unknown; handshake feasibility assumed but unproven.

**Capture guidance:**  
Direct join from lobby. 15–25 packets. Client + server versions.

---

## Garry's Mod

**Port:** UDP 27015  
**Known:** The connect channel is open and the Steam-auth gate appears to be absent on `sv_lan 1` deployments. The A2S query and challenge exchange are fully decoded. Two sequential gates were passed during iterative reverse-engineering:

1. **Challenge structure:** `A2S_GETCHALLENGE` (packet `ffffffff71`) returns:

| field | value | meaning |
|---|---|---|
| magic | `33494f5a` (LE) | `S2C_MAGICVERSION` constant — **not** the challenge |
| challenge | `3c7127b1` (LE) | **the actual challenge** (2nd uint32), stable across repeated requests |
| unknown | `00000000` | purpose unknown |
| auth protocol | `03000000` | 3 = PROTOCOL_STEAM; server requires Steam auth in theory, but LAN mode (`sv_lan 1`) appears to bypass it |
| steam2 key size | `0000` | no encryption key |
| server steamID64 | `115035c6e8c54001` (LE) | unique server identifier |
| secure flag | `00` | 0 = not VAC-secure, expected on LAN |
| trailing | `30303030303000` | `"000000\0"` |

2. **Gate progression:** Probes revealed that wrong field order was rejected with `#GameUI_ServerRejectOldVersion`, and correct field order moved the rejection to `#GameUI_ServerRejectBadChallenge`.

**Blocking:** Reading the challenge from the correct field advanced join to version negotiation, which fails. Bisecting the version field converged on two adjacent values (`2729496038` → `Old`, `2729496039` → `New`) with no accepted value between them. This pattern proves the field being probed is **not** the version field itself — the actual version lies in a different offset, and the exact `C2S_CONNECT` packet layout cannot be derived by blind iteration.

**Capture guidance:**  
Record the join handshake on **UDP 27015**, client→server and server→client, from `A2S_GETCHALLENGE` through the **`C2S_CONNECT` packet** that the real client sends. We can already produce everything up to that packet; the capture's prize is the single `C2S_CONNECT` payload.

**Critical:** Record the exact byte layout:
- Field order, byte widths, and alignment
- Which fields are null-terminated strings vs. length-prefixed
- Any nested or variable-length structures

Also provide:
- **Client build:** Exact GMod client version (from launcher or binary)
- **Server build:** Exact server version (visible in `ceifa/garrysmod` logs)
- **Server config:** Confirm `sv_lan 1` was set (so the capture represents LAN-mode behavior, without Steam auth)

---

## Valheim

**Port:** UDP 2456  
**⚠️ Known blocker:** Valheim joins route through **Steam Datagram Relay (SDR)**. The join begins with Steam authentication; the game port may not be directly reachable by a non-Steam client at all. A headless join without Steam identity may be architecturally impossible, not just undocumented.

**Blocking:** Transport and authentication are Steam-integrated, not standalone.

**Capture guidance:**  
Capture from a Steam client joining a Valheim server. This will show whether the traffic goes through SDR (forwarded via Steam) or direct UDP. 15–25 packets. Client + server versions. The capture is valuable for confirming the architectural constraint, but expect the evidence to suggest this game is not joinable without Steam.

---

## DayZ

**Port:** UDP 2302 (A2S on 27015)  
**⚠️ Known blocker:** **BattlEye anti-cheat gates the join.** The anti-cheat validates the game binary, checksums, and runtime memory. Even a fully documented Enfusion protocol would not permit a headless client (which lacks a GPU driver, display server, audio stack, and game binary) to pass validation. This is not a documentation gap — it is architectural.

**Recommendation:** Consider skipping this capture unless you want the evidence for documentation purposes. The blocker is anti-cheat policy, not protocol secrecy. If you do capture it, expect to see the join rejected at the anti-cheat stage, not the game protocol stage.

**Capture guidance (if attempted):**  
Join attempt. Anti-cheat will likely reject before the game handshake completes. 10–20 packets. Note the exact rejection point (which service rejects, at what phase) and client + server versions.

---

## Capture Recipe

Run this on a machine with network access to the target server:

```bash
tcpdump -i <interface> -w <game>-join.pcap 'udp and (dst host <server-ip> and dst port <port>) or (src host <server-ip> and src port <port>)'
```

Replace `<interface>` (e.g., `eth0`, `wlan0`), `<game>`, `<server-ip>`, and `<port>` as needed. Start the capture before opening the game join dialog. Stop after the server accepts the join or rejects with a visible error.

**Typical handshake duration:** 0.5–2 seconds from first packet to acceptance/rejection. You will not need to capture long — if 30 seconds of traffic have elapsed with no visible join/rejection, stop and check your filter.

### What makes a capture unusable

- **Session nonces / derived keys:** If every byte after the handshake is encrypted with session-specific keys, the capture shows only the handshake. That is fine — the handshake is what we need. But note: if every join uses a fresh symmetric key derived from the handshake, replaying the capture yields nothing; we will need to understand the derivation.
- **Encryption from byte 0:** Satisfactory uses TLS-over-UDP. If your target game encrypts all traffic before the game protocol appears, the raw pcap is unreadable. (We would need the server's key material or a man-in-the-middle proxy, which defeats the purpose. Escalate to the developers or skip.)
- **Anti-cheat interleaving:** DayZ and similar games run anti-cheat handshakes in parallel with game protocol. The capture will show both; we can filter to the game protocol if the anti-cheat doesn't reject first.
- **Version-locked protocols:** Factorio and Valheim are version-locked. If the capture is from a different server/client version than what we will test against in CI, the bytes may be uninterpretable or rejected. Always record the exact versions alongside the pcap.

### What to provide per game

For each capture:

1. **The pcap file** (e.g., `factorio-join.pcap`)
2. **Exact versions:**  
   - Game client version (from launcher, binary, or version string in logs)
   - Server version (same source)
3. **Join outcome:**  
   - Did the join succeed? Or was it rejected?
   - If rejected, what was the error message or code?

---

## Games NOT in this file

These 5 games have partial public documentation and are being reverse-engineered iteratively from specs. They are **not** on this list because we have a starting point:

- **Rust:** RakNet is open-source; packet byte layout is documented (but our implementation is incomplete).
- **CS2:** ⚠️ **Now expected to be worse than Source 1.** Source 2's connect packet reportedly carries embedded protobuf, a Steam auth ticket, and a reservation cookie. The Garry's Mod findings (which show hidden field offsets preventing blind iteration on Source 1) will not transfer; Source 2 is likely even more opaque. Partial documentation exists; captures would be more valuable here than for GMod.
- **Ark: Survival Ascended:** UE5 engine. Unreal Engine network stack is partially documented; iterating on join.
- **Palworld:** Unreal Engine 5. Same as Ark; iterative implementation in progress.
- **Satisfactory:** UE5 stateless-connect handshake. Concept documented by Epic; exact byte sequence under reverse-engineering.

Of these 5, **Rust, Ark, Palworld, and Satisfactory** have enough public information to attempt an implementation without a capture. Captures would accelerate them, but they are not blockers. **CS2 is now expected to require a capture** given Source 2's architectural differences from Source 1.
