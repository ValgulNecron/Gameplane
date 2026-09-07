import { useState } from "react";
import { useMutation } from "@tanstack/react-query";
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
  Input,
  Label,
  Description,
  Checkbox,
} from "@heroui/react";
import type { Role, PermissionGroup } from "@/types";
import { Roles } from "@/lib/endpoints";

export interface RoleEditorModalProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  role: Role | null;
  groups: PermissionGroup[];
  onSaved?: () => void;
}

export function RoleEditorModal({
  open,
  onOpenChange,
  role,
  groups,
  onSaved,
}: RoleEditorModalProps) {
  const creating = role === null;
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedPerms, setSelectedPerms] = useState<Set<string>>(new Set());

  // Reset state when dialog opens/closes
  const [resetFor, setResetFor] = useState<{ open: boolean; roleId: string | null }>({
    open: false,
    roleId: role?.name ?? null,
  });

  if (open !== resetFor.open || role?.name !== resetFor.roleId) {
    setResetFor({ open, roleId: role?.name ?? null });
    if (open) {
      setName(role?.name ?? "");
      setDescription(role?.description ?? "");
      setSelectedPerms(new Set(role?.permissions ?? []));
    }
  }

  const save = useMutation({
    mutationFn: () => {
      const permissions = Array.from(selectedPerms);
      return role
        ? Roles.update(role.name, { description, permissions })
        : Roles.create({ name, description, permissions });
    },
    onSuccess: () => {
      onOpenChange(false);
      onSaved?.();
    },
  });

  const nameValid = /^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$/.test(name);
  const submitDisabled = (creating && !nameValid) || save.isPending;

  const togglePermission = (perm: string) => {
    setSelectedPerms((prev) => {
      const next = new Set(prev);
      if (next.has(perm)) {
        next.delete(perm);
      } else {
        next.add(perm);
      }
      return next;
    });
  };

  return (
    <Modal isOpen={open} onOpenChange={onOpenChange}>
      <ModalBackdrop isDismissable={!save.isPending} isKeyboardDismissDisabled={save.isPending} />
      <ModalContainer size="lg">
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>{role ? `Edit role: ${role.name}` : "New role"}</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <Description className="text-sm text-muted">
              Grant a curated set of permissions.
            </Description>

            {creating && (
              <div className="space-y-2">
                <Label htmlFor="role-name" className="text-xs">
                  Name
                </Label>
                <Input
                  id="role-name"
                  autoFocus
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="support"
                />
              </div>
            )}

            <div className="space-y-2">
              <Label htmlFor="role-description" className="text-xs">
                Description
              </Label>
              <Input
                id="role-description"
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Manage game servers, backups, templates, and schedules."
              />
            </div>

            <div className="border border-border rounded-md p-3 max-h-72 overflow-y-auto space-y-4">
              {groups.map((group) => (
                <div key={group.resource} className="space-y-2">
                  <div className="text-xs font-semibold text-muted uppercase tracking-wider">
                    {group.label}
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    {group.permissions.map((perm) => (
                      <Checkbox key={perm.key} isSelected={selectedPerms.has(perm.key)} onChange={() => togglePermission(perm.key)} isDisabled={save.isPending}>
                        <Checkbox.Control>
                          <Checkbox.Indicator />
                        </Checkbox.Control>
                        <Checkbox.Content>
                          <span className="font-mono text-sm">{perm.key}</span>
                          {perm.namespaced && (
                            <span className="text-xs text-muted bg-muted bg-opacity-25 px-1 py-0.5 rounded">
                              ns
                            </span>
                          )}
                        </Checkbox.Content>
                      </Checkbox>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              onPress={() => onOpenChange(false)}
              isDisabled={save.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              isDisabled={submitDisabled}
              onPress={() => save.mutate()}
            >
              {save.isPending ? "Saving…" : creating ? "Create role" : "Save role"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}
