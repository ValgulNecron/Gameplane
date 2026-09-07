import { Input, Button, Chip, Table } from "@heroui/react";
import { Lock, Plus, Type, Trash2 } from "lucide-react";
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
    <div className="space-y-4">
      {env.length === 0 && (
        <p className="text-sm text-default-500">
          No environment variables. Click below to add one.
        </p>
      )}

      {env.length > 0 && (
        <Table.Root>
          <Table.ScrollContainer>
            <Table.Content
              aria-label="Environment variables"
              className="mb-4 max-h-[400px]"
            >
              <Table.Header>
                <Table.Column key="type" width="80" isRowHeader>
                  Type
                </Table.Column>
                <Table.Column key="name" width="160">
                  Name
                </Table.Column>
                <Table.Column key="value" width="100%">
                  Value
                </Table.Column>
                <Table.Column key="actions" id="actions" width="40" className="text-end">
                  Actions
                </Table.Column>
              </Table.Header>
              <Table.Body>
                {env.map((v, idx) => {
                  const isSecret = !!v.valueFrom?.secretKeyRef;
                  const nameInvalid = v.name !== "" && !ENV_NAME.test(v.name);
                  const dup = v.name !== "" && dupes.has(v.name);
                  return (
                    <Table.Row key={idx}>
                      <Table.Cell>
                        <Chip
                          size="sm"
                          variant="soft"
                          className="text-xs"
                        >
                          {isSecret ? (
                            <>
                              <Lock className="h-3 w-3" />
                              Secret
                            </>
                          ) : (
                            <>
                              <Type className="h-3 w-3" />
                              Literal
                            </>
                          )}
                        </Chip>
                      </Table.Cell>
                      <Table.Cell>
                        <div className="space-y-1">
                          <Input

                            value={v.name}
                            onChange={(e) => update(idx, { ...v, name: e.target.value })}
                            placeholder="VAR_NAME"
                            spellCheck={false}
                            className="text-xs"
                          />
                          {nameInvalid && (
                            <div className="text-xs text-danger">
                              Must match [A-Z_][A-Z0-9_]*
                            </div>
                          )}
                          {dup && (
                            <div className="text-xs text-danger">
                              Duplicate name
                            </div>
                          )}
                        </div>
                      </Table.Cell>
                      <Table.Cell>
                        {isSecret ? (
                          <div className="space-y-2">
                            <div className="flex gap-2">
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
                                className="text-xs flex-1"
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
                                className="text-xs flex-1"
                              />
                            </div>
                          </div>
                        ) : (
                          <Input
                           
                            value={v.value ?? ""}
                            onChange={(e) => update(idx, { ...v, value: e.target.value })}
                            placeholder="value"
                            spellCheck={false}
                            className="text-xs"
                          />
                        )}
                      </Table.Cell>
                      <Table.Cell>
                        <Button
                          isIconOnly
                          size="sm"
                          variant="ghost"
                          aria-label="Remove"
                          onPress={() => remove(idx)}
                          className="text-danger hover:bg-danger/10"
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </Table.Cell>
                    </Table.Row>
                  );
                })}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table.Root>
      )}

      <div className="flex items-center gap-2">
        <Button
          size="sm"
          variant="outline"
          onPress={() => add("literal")}
        >
          <Plus className="h-4 w-4" />
          Add variable
        </Button>
        <Button
          size="sm"
          variant="outline"
          onPress={() => add("secret")}
        >
          <Lock className="h-4 w-4" />
          Add from secret
        </Button>
      </div>
    </div>
  );
}
