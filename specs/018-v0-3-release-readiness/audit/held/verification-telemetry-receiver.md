# T045 aux chunk: independent verification (opus)

Held (OD-019, off-git). Verifier input: `audit/held/review-telemetry-receiver.md` (opus reviewer), candidates H-telemetry-receiver-01 and -02.

**Method.** I read the ingest handler on `master` (`13a859ff`; the file is identical on this branch), `telemetry-receiver/main.go:117-152`, against the spec's validation contract (`specs.md:14`, `:45`, `:55-58`, `:76`) and the existing rejection tests (`main_test.go:92-118`). I also checked the API reporter's payload struct (`api/internal/telemetry/telemetry.go:59-63`). It always sends all three fields, so neither item affects real reports from a Gameplane API. I built the receiver from this tree into the session scratchpad, ran it on `127.0.0.1`, and sent well-formed and malformed bodies with `curl` to see which the validation control accepts. Afterwards I read `/metrics` to see what was counted. Neither item is tracked in `findings.md`: F-019 is about who can reach the receiver. I ran no test or lint suite. Severity follows research R3. Both items are S4: they affect only the accuracy of the aggregate metrics, and a sender can already submit any valid values.

| Candidate | Verdict (kept/rejected) | Severity | Reason |
|---|---|---|---|
| H-telemetry-receiver-01 | kept | S4 | Confirmed. The spec marks `version`, `servers` and `templates` as required (`specs.md:55-57`). The decoder rejects unknown fields and wrong types, but it has no presence check. `{}`, `null` and `{"version":"1.0"}` each got `204` and were counted: the absent counts were recorded as `0`, and the absent version under `version="invalid"`. |
| H-telemetry-receiver-02 | kept | S4 | Confirmed. The handler decodes one JSON value and never checks for trailing input (`main.go:125-136`). A valid object followed by non-JSON bytes got `204`. So did a valid object followed by about 20 KB of padding (a 20,143-byte body), where the spec promises `413` for any body over 16 KiB (`specs.md:14`, `:45`, `README.md:21`). An oversized value inside the object still gets `413`, and that is the only case the existing test covers (`main_test.go:103`). |

### H-telemetry-receiver-01

**Location:** `telemetry-receiver/main.go:125-140` (decode, then check only for negative counts), measured against `telemetry-receiver/specs.md:55-57`.

**Control:** input validation on `POST /ingest`. The spec says only a payload with exactly the three required fields is accepted and counted.

**Repro / observation** (how to confirm the control holds):
1. Read `main.go:125-140`. `json.Decoder` with `DisallowUnknownFields` fills a `payload` struct of plain `string`/`int` fields, and the only check after that is `Servers < 0 || Templates < 0`. The code never distinguishes an absent field from its zero value. A top-level `null` decodes into the zero struct without error.
2. Local check with the receiver built from this tree (`LISTEN_ADDR=127.0.0.1:28180`): POST bodies with one or more required fields missing, and a body that is the JSON literal `null`. On the current tree each gets `204`, and `/metrics` counts it.
3. Regression test the fix should add: extend `TestIngestRejects` with one case per missing field and one for a `null` document, each expecting `400`. Keep the existing assertion that nothing rejected was counted (`main_test.go:114-117`).

**Expected:** A payload that lacks any of the three fields, or isn't a JSON object, is rejected with `400` and not counted, as `specs.md:45` says for malformed payloads and `specs.md:55-57` says for required fields.

**Actual:** Such payloads get `204` and are counted, which skews the fleet-size histograms towards `0`.

### H-telemetry-receiver-02

**Location:** `telemetry-receiver/main.go:125-136` (a single `dec.Decode`, with no end-of-input check), measured against `telemetry-receiver/specs.md:14`, `:45` and `telemetry-receiver/README.md:21`.

**Control:** input validation and the 16 KiB body-size limit on `POST /ingest`.

**Repro / observation** (how to confirm the control holds):
1. Read `main.go:125-136`. `json.Decoder.Decode` returns as soon as the first complete JSON value has been read. `http.MaxBytesReader` raises its error only when a read goes past the limit, and for a small leading object the decoder never reads that far. Nothing after `Decode` checks that the body is exhausted.
2. Local check with the receiver built from this tree: POST a valid report followed by non-whitespace bytes, and a valid report followed by padding that takes the body over 16 KiB. On the current tree both get `204` and both are counted. A body whose oversized content sits inside the JSON value still gets `413`.
3. Regression test the fix should add: rejection cases for "valid object, then non-whitespace" (expect `400`) and "valid object, then more than 16 KiB of padding" (expect `413`), both asserted as not counted.

**Expected:** After decoding, the handler confirms the body holds nothing more, for example with a second `Decode` that must return `io.EOF`, reading far enough that `MaxBytesError` still maps to `413`. Trailing content gets `400`, and any body over 16 KiB gets `413`, as the spec and README say.

**Actual:** Only the first JSON value is validated. Trailing content, including content past the 16 KiB cap, is accepted with `204`.
