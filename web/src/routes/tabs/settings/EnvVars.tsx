import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import { Lock, Plus, Type, X } from "lucide-react";
import type { EnvVar } from "@/types";
import type { SectionProps } from "./types";

const ENV_NAME = /^[A-Z_][A-Z0-9_]*$/;

export function EnvVarsSection({ draft, onChange }: SectionProps) {
  const env = draft.spec.env ?? [];

  const setEnv = (next: EnvVar[]) => {
    onChange({
      ...draft,
      spec: { ...draft.spec, env: next.length ? next : undefined },
    });
  };

  const update = (idx: number, patch: EnvVar) => {
    setEnv(env.map((v, i) => (i === idx ? patch : v)));
  };
  const remove = (idx: number) => setEnv(env.filter((_, i) => i !== idx));
  const add = (kind: "literal" | "secret") => {
    if (kind === "literal") setEnv([...env, { name: "", value: "" }]);
    else setEnv([...env, { name: "", valueFrom: { secretKeyRef: { name: "", key: "" } } }]);
  };

  const seen = new Set<string>();
  const dupes = new Set<string>();
  for (const v of env) {
    if (v.name) {
      if (seen.has(v.name)) dupes.add(v.name);
      else seen.add(v.name);
    }
  }

  return (
    <div className="space-y-3">
      {env.length === 0 && (
        <p className="text-sm text-muted">
          No environment variables. Click below to add one.
        </p>
      )}
      {env.map((v, idx) => {
        const isSecret = !!v.valueFrom?.secretKeyRef;
        const nameInvalid = v.name !== "" && !ENV_NAME.test(v.name);
        const dup = v.name !== "" && dupes.has(v.name);
        return (
          <div
            key={idx}
            className="grid grid-cols-[24px_220px_1fr_32px] items-start gap-2 rounded border border-border bg-surface/30 p-2"
          >
            <div className="pt-2 text-muted">
              {isSecret ? <Lock className="h-3 w-3" /> : <Type className="h-3 w-3" />}
            </div>
            <div>
              <Input
                value={v.name}
                onChange={(e) => update(idx, { ...v, name: e.target.value })}
                placeholder="VAR_NAME"
                spellCheck={false}
                className={nameInvalid || dup ? "border-danger focus:ring-danger" : ""}
              />
              {nameInvalid && (
                <div className="pt-1 text-xs text-danger">
                  Must match [A-Z_][A-Z0-9_]*
                </div>
              )}
              {dup && (
                <div className="pt-1 text-xs text-danger">Duplicate name</div>
              )}
            </div>
            {isSecret ? (
              <div className="grid grid-cols-2 gap-2">
                <Input
                  value={v.valueFrom?.secretKeyRef?.name ?? ""}
                  onChange={(e) =>
                    update(idx, {
                      ...v,
                      valueFrom: {
                        secretKeyRef: {
                          name: e.target.value,
                          key: v.valueFrom?.secretKeyRef?.key ?? "",
                        },
                      },
                    })
                  }
                  placeholder="secret-name"
                  spellCheck={false}
                />
                <Input
                  value={v.valueFrom?.secretKeyRef?.key ?? ""}
                  onChange={(e) =>
                    update(idx, {
                      ...v,
                      valueFrom: {
                        secretKeyRef: {
                          name: v.valueFrom?.secretKeyRef?.name ?? "",
                          key: e.target.value,
                        },
                      },
                    })
                  }
                  placeholder="key"
                  spellCheck={false}
                />
              </div>
            ) : (
              <Input
                value={v.value ?? ""}
                onChange={(e) => update(idx, { ...v, value: e.target.value })}
                placeholder="value"
                spellCheck={false}
              />
            )}
            <Button
              variant="ghost"
              size="icon"
              className="h-8 w-8"
              title="Remove"
              onClick={() => remove(idx)}
            >
              <X className="h-3 w-3" />
            </Button>
          </div>
        );
      })}
      <div className="flex items-center gap-2">
        <Button size="sm" variant="outline" onClick={() => add("literal")}>
          <Plus className="h-3 w-3" /> Add variable
        </Button>
        <Button size="sm" variant="outline" onClick={() => add("secret")}>
          <Lock className="h-3 w-3" /> Add from secret
        </Button>
      </div>
    </div>
  );
}
