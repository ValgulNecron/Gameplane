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
import { ErrorBanner } from "@/components/ui/ErrorBanner";

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
  const [name, setName] = useState(() => role?.name ?? "");
  const [description, setDescription] = useState(() => role?.description ?? "");
  const [selectedPerms, setSelectedPerms] = useState<Set<string>>(
    () => new Set(role?.permissions ?? []),
  );

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
    <Modal isOpen={open} onOpenChange={onOpenChange} key={role?.name ?? "new"}>
      <ModalBackdrop isDismissable={!save.isPending} isKeyboardDismissDisabled={save.isPending}>
        <ModalContainer>
          <ModalDialog className="w-[480px] max-w-[480px]">
            <ModalHeader>
              <ModalHeading>{role ? `Edit role: ${role.name}` : "New role"}</ModalHeading>
              <Description className="text-sm leading-[18px] text-muted">
                Grant a curated set of permissions.
              </Description>
            </ModalHeader>

            <ModalBody className="gap-3">

            {save.error && <ErrorBanner err={save.error} onDismiss={() => save.reset()} />}

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

            <div className="flex flex-col gap-1.5">
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

            <div className="border border-border rounded-md p-3 h-72 overflow-y-auto space-y-3">
              {groups.map((group) => (
                <div key={group.resource} className="space-y-1">
                  <div className="text-[11px] leading-[14px] font-semibold text-muted uppercase tracking-[0.55px]">
                    {group.label}
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    {group.permissions.map((perm) => (
                      <Checkbox
                        key={perm.key}
                        isSelected={selectedPerms.has(perm.key)}
                        onChange={() => togglePermission(perm.key)}
                        isDisabled={save.isPending}
                        className="flex-row items-center gap-2"
                      >
                        <Checkbox.Control className="size-4">
                          <Checkbox.Indicator />
                        </Checkbox.Control>
                        <Checkbox.Content>
                          <span className="font-mono text-xs">{perm.key}</span>
                          {perm.namespaced && (
                            <span className="text-[9px] text-muted bg-[#94949426] px-1 rounded">
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
              variant="ghost"
              size="sm"
              className="h-9 px-4"
              onPress={() => onOpenChange(false)}
              isDisabled={save.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              className="h-9 px-4"
              isDisabled={submitDisabled}
              onPress={() => save.mutate()}
            >
              {save.isPending ? "Saving…" : creating ? "Create role" : "Save role"}
            </Button>
            </ModalFooter>
          </ModalDialog>
        </ModalContainer>
      </ModalBackdrop>
    </Modal>
  );
}
