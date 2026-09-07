import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import {
  Modal,
  ModalBackdrop,
  ModalContainer,
  ModalDialog,
  ModalHeader,
  ModalHeading,
  ModalBody,
  ModalFooter,
  Button,
  TextField,
  Input,
  Label,
  Description,
  FieldError,
} from "@heroui/react";
import { Servers } from "@/lib/endpoints";
import { APIError } from "@/lib/api";
import { isValidK8sName } from "@/lib/validation";
import { errorText } from "@/lib/errors";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sourceName: string;
  ns?: string;
  onCloned?: () => void;
}

function cloneErrorMessage(err: unknown, name: string): string {
  if (err instanceof APIError) {
    if (err.status === 409) return `A server named ${name} already exists.`;
    if (err.status === 403) return "Your role does not allow cloning servers.";
    return err.body.slice(0, 240) || `Clone failed (${err.status}).`;
  }
  return errorText(err, "Unknown error");
}

export function CloneServerDialog({
  open,
  onOpenChange,
  sourceName,
  ns,
  onCloned,
}: Props) {
  const qc = useQueryClient();
  const nav = useNavigate();
  const [newName, setNewName] = useState("");
  // Reset the suggested name whenever the dialog (re)opens or the source
  // server changes. Adjusted directly during render (not in an effect),
  // gated on the previously-seen (open, sourceName) pair.
  const [resetFor, setResetFor] = useState<{ open: boolean; sourceName: string }>({
    open: false,
    sourceName,
  });
  if (open !== resetFor.open || sourceName !== resetFor.sourceName) {
    setResetFor({ open, sourceName });
    if (open) setNewName(`${sourceName.slice(0, 58)}-copy`);
  }

  const clone = useMutation({
    mutationFn: () => Servers.clone(sourceName, newName, ns),
    onSuccess: async (created) => {
      await qc.invalidateQueries({ queryKey: ["servers"] });
      await qc.invalidateQueries({ queryKey: ["my-servers"] });
      onOpenChange(false);
      onCloned?.();
      await nav({ to: "/servers/$name", params: { name: created.metadata.name } });
    },
  });

  const valid = isValidK8sName(newName);

  return (
    <Modal isOpen={open} onOpenChange={onOpenChange}>
      <ModalBackdrop isDismissable={!clone.isPending} isKeyboardDismissDisabled={clone.isPending} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Clone server</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <Description className="text-sm text-muted">
              Creates a new server with the same configuration. World data is not copied.
            </Description>

            <TextField isInvalid={!valid || clone.isError}>
              <Label className="text-xs">New name</Label>
              <Input
                autoFocus
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                spellCheck={false}
                placeholder="mc-survival-copy"
                className="mt-1"
              />
              {!valid && (
                <FieldError className="mt-1 text-xs">
                  Name must be lowercase letters, digits, dashes (max 63)
                </FieldError>
              )}
              {clone.isError && (
                <FieldError className="mt-1 text-xs">
                  {cloneErrorMessage(clone.error, newName)}
                </FieldError>
              )}
            </TextField>
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              onPress={() => onOpenChange(false)}
              isDisabled={clone.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              isDisabled={!valid || clone.isPending}
              onPress={() => clone.mutate()}
            >
              {clone.isPending ? "Cloning…" : "Clone server"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}
