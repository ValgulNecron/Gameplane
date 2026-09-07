import { useEffect, useState } from "react";
import Editor from "@monaco-editor/react";
import { Label, Description } from "@heroui/react";
import type { SectionProps } from "./types";

interface PlacementSectionProps extends SectionProps {
  onValidityChange?: (valid: boolean) => void;
}

export function PlacementSection({
  draft,
  onChange,
  onValidityChange,
}: PlacementSectionProps) {
  // Initialize raw JSON strings once via useState initializers.
  const [rawTol, setRawTol] = useState(() =>
    JSON.stringify(draft.spec.tolerations ?? [], null, 2),
  );
  const [rawAff, setRawAff] = useState(() =>
    JSON.stringify(draft.spec.affinity ?? {}, null, 2),
  );

  const [tolError, setTolError] = useState<string | null>(null);
  const [affError, setAffError] = useState<string | null>(null);

  // Report validity when errors change.
  useEffect(() => {
    onValidityChange?.(tolError === null && affError === null);
  }, [tolError, affError, onValidityChange]);

  const handleTolChange = (v: string | undefined) => {
    const raw = v ?? "";
    setRawTol(raw);

    if (raw.trim() === "") {
      setTolError(null);
      onChange({
        ...draft,
        spec: { ...draft.spec, tolerations: undefined },
      });
      return;
    }

    try {
      const parsed = JSON.parse(raw);
      if (!Array.isArray(parsed)) {
        setTolError("Must be a JSON array");
        return;
      }
      setTolError(null);
      onChange({
        ...draft,
        spec: {
          ...draft.spec,
          tolerations: parsed.length ? parsed : undefined,
        },
      });
    } catch {
      setTolError("Invalid JSON");
    }
  };

  const handleAffChange = (v: string | undefined) => {
    const raw = v ?? "";
    setRawAff(raw);

    if (raw.trim() === "") {
      setAffError(null);
      onChange({
        ...draft,
        spec: { ...draft.spec, affinity: undefined },
      });
      return;
    }

    try {
      const parsed = JSON.parse(raw);
      if (typeof parsed !== "object" || Array.isArray(parsed) || parsed === null) {
        setAffError("Must be a JSON object");
        return;
      }
      setAffError(null);
      onChange({
        ...draft,
        spec: {
          ...draft.spec,
          affinity: Object.keys(parsed).length ? parsed : undefined,
        },
      });
    } catch {
      setAffError("Invalid JSON");
    }
  };

  return (
    <div className="space-y-6">
      <div className="grid grid-cols-1 items-start gap-1.5 sm:grid-cols-[200px_1fr] sm:gap-4">
        <div className="sm:pt-2">
          <Label htmlFor="tolerations-editor" className="text-sm">
            Tolerations
          </Label>
          <Description className="pt-1 text-xs">
            Pod tolerations for Kubernetes node taints. Array of toleration objects (optional).
          </Description>
        </div>
        <div className="space-y-2">
          <div
            id="tolerations-editor"
            className="rounded border border-border bg-surface/50"
            style={{ height: "180px" }}
          >
            <Editor
              theme="vs-dark"
              language="json"
              value={rawTol}
              onChange={handleTolChange}
              options={{
                minimap: { enabled: false },
                fontFamily: "JetBrains Mono",
              }}
            />
          </div>
          {tolError && <div className="pt-1 text-xs text-danger">{tolError}</div>}
        </div>
      </div>

      <div className="grid grid-cols-1 items-start gap-1.5 sm:grid-cols-[200px_1fr] sm:gap-4">
        <div className="sm:pt-2">
          <Label htmlFor="affinity-editor" className="text-sm">
            Affinity
          </Label>
          <Description className="pt-1 text-xs">
            Pod affinity/anti-affinity and node affinity constraints. JSON object (optional).
          </Description>
        </div>
        <div className="space-y-2">
          <div
            id="affinity-editor"
            className="rounded border border-border bg-surface/50"
            style={{ height: "180px" }}
          >
            <Editor
              theme="vs-dark"
              language="json"
              value={rawAff}
              onChange={handleAffChange}
              options={{
                minimap: { enabled: false },
                fontFamily: "JetBrains Mono",
              }}
            />
          </div>
          {affError && <div className="pt-1 text-xs text-danger">{affError}</div>}
        </div>
      </div>
    </div>
  );
}
