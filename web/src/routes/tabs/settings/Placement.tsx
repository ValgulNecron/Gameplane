import { useEffect, useState } from "react";
import Editor from "@monaco-editor/react";
import { Field } from "./Field";
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
      <Field
        label="Tolerations"
        hint="Pod tolerations for Kubernetes node taints. Array of toleration objects (optional)."
      >
        <div className="space-y-2">
          <div
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
      </Field>

      <Field
        label="Affinity"
        hint="Pod affinity/anti-affinity and node affinity constraints. JSON object (optional)."
      >
        <div className="space-y-2">
          <div
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
      </Field>
    </div>
  );
}
