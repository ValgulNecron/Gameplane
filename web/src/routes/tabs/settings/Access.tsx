import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { X } from "lucide-react";
import type { GameServer } from "@/types";
import { Button, Input, Label, Chip, Card, CardContent } from "@heroui/react";
import { Servers } from "@/lib/endpoints";
import { errorText } from "@/lib/errors";
import { useMe, can } from "@/lib/auth";
import { OWNER_ID_ANNOTATION, OWNER_ANNOTATION } from "@/lib/annotations";

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

  const setCollab = useMutation({
    mutationFn: async (body: { userIds?: number[]; usernames?: string[] }) => {
      if (!gs) return;
      const namespace = gs.metadata.namespace ?? "gameplane-games";
      return Servers.setCollaborators(gs.metadata.name, namespace, body);
    },
    onSuccess: () => {
      setAddInput("");
      setError(null);
      return qc.invalidateQueries({ queryKey: ["server", gs?.metadata.name] });
    },
    onError: (err) => {
      setError(errMsg(err));
    },
  });

  if (!gs) {
    return <div className="p-6 text-sm text-default-500">Loading…</div>;
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

  // Misalignment guard: if the parsed collaborators and collaborator-names
  // have different lengths, the annotations were modified outside the dashboard.
  const isAligned = collaboratorIDs.length === collaboratorNames.length;
  const canEditCollaborators = canManage && isAligned;

  const handleAddCollaborator = () => {
    const name = addInput.trim();
    if (!name) return;

    // Keep existing IDs; send only the new username.
    // The server unions and dedupes.
    const newUserIds = collaboratorIDs.map((id) => Number(id));

    setCollab.mutate({
      userIds: newUserIds,
      usernames: [name],
    });
  };

  const handleRemoveCollaborator = (index: number) => {
    // Remove the collaborator at the given index
    const newUserIds = collaboratorIDs
      .filter((_, i) => i !== index)
      .map((id) => Number(id));

    setCollab.mutate({
      userIds: newUserIds,
    });
  };

  return (
    <div className="space-y-3">
      <Card className="border border-divider bg-default/40">
        <CardContent className="gap-0 px-4 py-4">
          <Label className="text-sm font-medium">Owner</Label>
          <div className="pt-1 font-mono text-sm text-default-500">{ownerName}</div>
        </CardContent>
      </Card>

      <Card className="border border-divider bg-default/40">
        <CardContent className="gap-0 px-4 py-4">
          <div className="space-y-3">
            <div>
              <Label className="text-sm font-medium">Collaborators</Label>
              <p className="pt-1 text-xs text-default-500">
                Collaborators get full control of this server (console, files, settings). They
                can&apos;t transfer ownership or edit this list.
              </p>
            </div>

            <div className="space-y-2">
              {!isAligned && (
                <div className="rounded border border-warning/40 bg-warning/10 px-2 py-1 text-xs text-warning-600 dark:text-warning-400">
                  Collaborators were modified outside the dashboard.
                </div>
              )}
              {collaboratorNames.length > 0 ? (
                <div className="flex flex-wrap gap-2">
                  {collaboratorNames.map((name, idx) => (
                    <Chip
                      key={idx}
                      variant="soft"
                      color="default"
                      className="flex items-center gap-1"
                    >
                      <span>{name}</span>
                      {canEditCollaborators && (
                        <button
                          onClick={() => handleRemoveCollaborator(idx)}
                          disabled={setCollab.isPending}
                          className="ml-0.5 hover:text-danger disabled:opacity-40"
                          title="Remove"
                          aria-label={`Remove ${name}`}
                        >
                          <X className="h-3 w-3" />
                        </button>
                      )}
                    </Chip>
                  ))}
                </div>
              ) : (
                <div className="text-xs text-default-500">None yet</div>
              )}
            </div>

            {canEditCollaborators && (
              <div className="flex items-center gap-2 pt-1">
                <Input
                  value={addInput}
                  onChange={(e) => setAddInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      handleAddCollaborator();
                    }
                  }}
                  placeholder="Add collaborator by username…"
                  className="flex-1"
                  disabled={setCollab.isPending}
                  type="text"
                />
                <Button
                  size="sm"
                  variant="primary"
                  onPress={() => handleAddCollaborator()}
                  isDisabled={!addInput.trim() || setCollab.isPending}
                >
                  Add
                </Button>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {error && (
        <div className="rounded border border-danger/40 bg-danger/10 px-3 py-2 text-xs text-danger">
          {error}
        </div>
      )}
    </div>
  );
}

function errMsg(err: unknown): string {
  return errorText(err, "failed");
}
