# Held review candidates: capture-sidecar (OD-019, off-git)

## Moved from audit/evidence/review-capture-sidecar/notes.md (OD-019, 2026-09-24)

### C-capture-sidecar-01: A capture started without a filter always fails because nobody fills in the default filter
- **Location**: `operator/internal/controller/networkcapture_controller.go:373-381`; `api/internal/handlers/capture.go:255-258`; `capture-sidecar/internal/httpserver/handlers.go:378-381`
- **Category**: correctness
- **Suggested severity**: S3
- **Observation / repro**:
  1. In the dashboard capture form, leave "Packet Filter" empty. The form labels it "Optional" (`CaptureWidget.tsx:588`) and sends `filter: undefined` (`:523`). An API client can equally omit `filter`.
  2. The API stores `spec.filter = nil` (`capture.go:255-258`). The CRD doc promises: "Optional. If omitted, a default filter restricts capture to the GameServer's own advertised ports." (`networkcapture_types.go:40`)
  3. The operator passes `nc.Spec.Filter` (nil) to `StartCapture` unchanged (`networkcapture_controller.go:373-381`), and `startCaptureRequest.Filter` is `omitempty` (`sidecar_capture.go:71`). No code in `operator/` or `api/` assigns a default filter; grepping for `Spec.Filter` / `.Filter =` finds only reads.
  4. The sidecar answers `400 filter is required: the control plane must supply the default port filter` (`handlers.go:378-381`). `IsTransientError` treats 4xx as permanent (`sidecar_capture.go:57-59`), so the operator calls `r.fail(...)` and the NetworkCapture goes to Failed.
- **Expected**: Contract `capture-sidecar.md:250` says: "FR-003 makes the filter optional at the *API* boundary … only the control plane knows those ports, so it must materialise that default before calling the sidecar." The capture should run with the advertised-ports filter.
- **Actual**: Every capture started without a filter fails. The workaround is to always type an explicit filter.

### Sentence replaced in the Observations bullet on e2e capture filters

The no-filter path has no e2e coverage.
