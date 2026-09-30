# Held review candidates: telemetry-receiver (OD-019, off-git)

- **Date**: 2026-09-24
- **Reviewer tier**: opus (verification pending)
- **Checked against**: `telemetry-receiver/specs.md` (Responsibilities, Ingest payload, Key invariants, Security considerations), `telemetry-receiver/README.md`
- **Companion notes**: `audit/evidence/review-telemetry-receiver/notes.md` (non-security candidates)

## Held candidates

### H-telemetry-receiver-01: Ingest validation doesn't enforce the "required" payload fields

- **Control**: input validation on `POST /ingest`.
- **Location**: `telemetry-receiver/main.go:125-140`
- **Category**: correctness
- **Suggested severity**: S4 (affects the accuracy of aggregate metrics; no confidentiality or integrity impact beyond the metrics themselves)
- **Spec statement**: `specs.md:55-57`: "**version** (string, required) ... **servers** (integer, required) ... **templates** (integer, required)". `specs.md:14`: "strict JSON schema (`{version, servers, templates}` only)". `specs.md:76`: "**Strict validation**: JSON decoder enforces exact field list".
- **Observation**: The decoder rejects unknown fields, wrong types and negative counts. It has no presence check: Go's zero values are accepted for absent fields, and a JSON `null` document decodes into the zero `payload` without error. A payload missing some or all of the three fields is therefore accepted (204) and counted, with absent counts recorded as `0` in both histograms and an absent version counted under `version="invalid"`.
- **Correct behaviour**: A payload without all three fields, or a non-object document, is rejected with `400` and not counted, as `specs.md:45` lists for malformed payloads.
- **How a maintainer confirms it holds**: Add table cases to `TestIngestRejects` for a document missing each field and for a `null` document, each expecting `400`, and keep the existing "nothing above must have been counted" assertion (`main_test.go:114-117`).

### H-telemetry-receiver-02: Only the first JSON value is validated; trailing content, including content past the 16 KiB cap, isn't checked

- **Control**: input validation and body-size limit on `POST /ingest`.
- **Location**: `telemetry-receiver/main.go:125-136`
- **Category**: correctness
- **Suggested severity**: S4
- **Spec statement**: `specs.md:14`: "rejects unknown fields, malformed JSON, oversized bodies (>16 KiB), and negative counts". `specs.md:45`: "`413` body >16 KiB". `README.md:21`: "`413` body over 16 KiB".
- **Observation**: `json.Decoder.Decode` returns once the first complete JSON value has been read, and the handler never checks whether more input follows. `http.MaxBytesReader` only raises its error when a read goes past the limit. If the leading object is small and valid, the decoder never reads that far. So a body made of a valid object followed by arbitrary non-JSON bytes, or followed by enough extra bytes to exceed 16 KiB, is accepted with `204` instead of `400`/`413`. The existing oversized test (`main_test.go:103`) puts the excess inside the first value, so it doesn't cover this.
- **Correct behaviour**: After decoding, the handler confirms the body is exhausted (for example `dec.More()` / a second `Decode` expecting `io.EOF`, reading to the limit so `MaxBytesError` still maps to `413`). Trailing content gives `400`, and any body over 16 KiB gives `413`, as the spec says.
- **How a maintainer confirms it holds**: Add rejection cases for "valid object followed by non-whitespace" (expect `400`) and "valid object followed by more than 16 KiB of padding" (expect `413`), and check that neither is counted.

## Questions (not findings)

- **No read deadline on bodies.** The HTTP server sets only `ReadHeaderTimeout` (`main.go:162-166`), so there is no time bound on reading a request body. specs.md makes no read-deadline claim for this component, so it isn't a spec contradiction. But `main.go:9-11` and the README present the receiver as something that can run "on a public host collecting reports from many installs". Is a body read timeout wanted for that deployment?
