import { useState } from "react";
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
  Select,
  ListBox,
  ListBoxItem,
  FieldError,
} from "@heroui/react";

export interface EditUserDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  displayName?: string;
  email?: string;
  role?: string;
  roles?: string[];
  onSave?: (displayName: string, email: string, role: string) => void;
  isLoading?: boolean;
}

export function EditUserDialog({
  open,
  onOpenChange,
  displayName: initialDisplayName = "",
  email: initialEmail = "",
  role: initialRole = "viewer",
  roles = ["admin", "operator", "viewer"],
  onSave,
  isLoading = false,
}: EditUserDialogProps) {
  const [displayName, setDisplayName] = useState(initialDisplayName);
  const [email, setEmail] = useState(initialEmail);
  const [role, setRole] = useState(initialRole);
  const [error, setError] = useState<string>("");

  const handleSave = () => {
    setError("");
    if (!displayName.trim()) {
      setError("Display name is required");
      return;
    }
    if (!email.trim()) {
      setError("Email is required");
      return;
    }
    if (!role) {
      setError("Role is required");
      return;
    }
    onSave?.(displayName, email, role);
  };

  const handleClose = () => {
    setDisplayName(initialDisplayName);
    setEmail(initialEmail);
    setRole(initialRole);
    setError("");
    onOpenChange(false);
  };

  return (
    <Modal isOpen={open} onOpenChange={handleClose}>
      <ModalBackdrop isDismissable={!isLoading} isKeyboardDismissDisabled={isLoading} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Edit user</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <div>
              <Label htmlFor="edit-display-name" className="text-xs">
                Display name
              </Label>
              <Input
                id="edit-display-name"
                autoFocus
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="Display name"
                className="mt-1"
                disabled={isLoading}
              />
            </div>

            <div>
              <Label htmlFor="edit-email" className="text-xs">
                Email
              </Label>
              <Input
                id="edit-email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="email@example.com"
                className="mt-1"
                type="email"
                disabled={isLoading}
              />
            </div>

            <div className="space-y-1">
              <Select
                value={role}
                onChange={(v) => setRole(String(v))}
                isDisabled={isLoading}
              >
                <Label htmlFor="edit-role" className="text-xs">
                  Primary role (cluster-wide)
                </Label>
                <Select.Trigger
                  id="edit-role"
                  className="w-full rounded border border-border bg-surface px-3 py-2 text-sm hover:bg-surface/80"
                >
                  <Select.Value />
                  <Select.Indicator className="ml-auto h-4 w-4" />
                </Select.Trigger>
                <Select.Popover className="rounded border border-border">
                  <ListBox className="p-0" aria-label="Primary role">
                    {roles.map((r) => (
                      <ListBoxItem key={r} id={r}>
                        {r}
                      </ListBoxItem>
                    ))}
                  </ListBox>
                </Select.Popover>
              </Select>
            </div>

            {error && (
              <FieldError className="text-xs">
                {error}
              </FieldError>
            )}
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              onPress={handleClose}
              isDisabled={isLoading}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              isDisabled={isLoading}
              onPress={handleSave}
            >
              {isLoading ? "Saving…" : "Save changes"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}
