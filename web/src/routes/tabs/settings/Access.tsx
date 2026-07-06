import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { X } from "lucide-react";
import type { GameServer } from "@/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Servers } from "@/lib/endpoints";
import { APIError } from "@/lib/api";
import { useMe, can } from "@/lib/auth";

const OWNER_ID_ANNOTATION = "gameplane.local/owner-id";
const OWNER_ANNOTATION = "gameplane.local/owner";
const COLLABORATORS_ANNOTATION = "gameplane.local/collaborators";
const COLLABORATOR_NAMES_ANNOTATION = "gameplane.local/collaborator-names";

interface Props {
  gs?: GameServer;
}

export function AccessSection({ gs }: Props) {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const [addInput, setAddInput] = useState("");
  const [error, setError] = useState<string | null>(null);

  if (!gs) {
    return <div className="p-6 text-sm text-muted">Loading…</div>;
  }

  const ann = gs.metadata.annotations ?? {};
  const ownerName = ann[OWNER_ANNOTATION] ?? "—";
  const ownerID = ann[OWNER_ID_ANNOTATION];
  const collaboratorIDs = ann[COLLABORATORS_ANNOTATION]
    ? ann[COLLABORATORS_ANNOTATION].split(",").map((s) => s.trim())
    : [];
  const collaboratorNames = ann[COLLABORATOR_NAMES_ANNOTATION]
    ? ann[COLLABORATOR_NAMES_ANNOTATION].split(",").map((s) => s.trim())
    : [];

  // Permission check: owner or servers:write
  const namespace = gs.metadata.namespace ?? "gameplane-games";
  const canManage = ownerID === String(me?.id) || can(me, "servers:write", namespace);

  const setCollab = useMutation({
    mutationFn: async (body: { userIds?: number[]; usernames?: string[] }) => {
      if (!gs) return;
      return Servers.setCollaborators(gs.metadata.name, namespace, body);
    },
    onSuccess: () => {
      setAddInput("");
      setError(null);
      void qc.invalidateQueries({ queryKey: ["server", gs.metadata.name] });
    },
    onError: (err) => {
      setError(errMsg(err));
    },
  });

  const handleAddCollaborator = async () => {
    const name = addInput.trim();
    if (!name) return;

    // Build the new collaborators list: keep existing IDs + add the new username
    const newUsernames = [...collaboratorNames, name];
    const newUserIds = collaboratorIDs.map((id) => Number(id));

    await setCollab.mutateAsync({
      userIds: newUserIds,
      usernames: newUsernames,
    });
  };

  const handleRemoveCollaborator = async (index: number) => {
    // Remove the collaborator at the given index
    const newUserIds = collaboratorIDs
      .filter((_, i) => i !== index)
      .map((id) => Number(id));

    await setCollab.mutateAsync({
      userIds: newUserIds,
    });
  };

  return (
    <div className="space-y-3">
      <Card>
        <div className="flex items-start justify-between gap-4">
          <div className="min-w-0">
            <div className="text-sm font-medium text-fg">Owner</div>
            <div className="pt-1 font-mono text-sm text-muted">{ownerName}</div>
          </div>
        </div>
      </Card>

      <Card>
        <div className="space-y-3">
          <div>
            <div className="text-sm font-medium text-fg">Collaborators</div>
            <div className="pt-1 text-xs text-muted">
              Collaborators get full control of this server (console, files, settings). They
              can't transfer ownership or edit this list.
            </div>
          </div>

          <div className="space-y-2">
            {collaboratorNames.length > 0 ? (
              <div className="flex flex-wrap gap-2">
                {collaboratorNames.map((name, idx) => (
                  <div
                    key={idx}
                    className="flex items-center gap-1 rounded-full border border-border bg-surface/60 px-2 py-1 text-xs"
                  >
                    <span>{name}</span>
                    {canManage && (
                      <button
                        onClick={() => void handleRemoveCollaborator(idx)}
                        disabled={setCollab.isPending}
                        className="ml-0.5 hover:text-danger disabled:opacity-40"
                        title="Remove"
                      >
                        <X className="h-3 w-3" />
                      </button>
                    )}
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-xs text-muted">None yet</div>
            )}
          </div>

          {canManage && (
            <div className="flex items-center gap-2 pt-1">
              <Input
                value={addInput}
                onChange={(e) => setAddInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    void handleAddCollaborator();
                  }
                }}
                placeholder="Add collaborator by username…"
                className="flex-1"
                disabled={setCollab.isPending}
              />
              <Button
                size="sm"
                onClick={() => void handleAddCollaborator()}
                disabled={!addInput.trim() || setCollab.isPending}
              >
                Add
              </Button>
            </div>
          )}
        </div>
      </Card>

      {error && (
        <div className="rounded border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {error}
        </div>
      )}
    </div>
  );
}

function Card({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex items-start justify-between gap-4 rounded border border-border bg-surface/30 p-4">
      <div className="min-w-0 flex-1">{children}</div>
    </div>
  );
}

function errMsg(err: unknown): string {
  if (err instanceof APIError) {
    try {
      const parsed = JSON.parse(err.body) as { error?: string };
      if (parsed.error) return parsed.error;
    } catch {
      // fall through
    }
    return err.body || `request failed (${err.status})`;
  }
  return err instanceof Error ? err.message : "failed";
}
