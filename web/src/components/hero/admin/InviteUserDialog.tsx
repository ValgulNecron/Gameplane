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
  FieldError,
} from "@heroui/react";

export interface InviteUserDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onInvite?: (username: string, displayName: string, email: string, password: string) => void;
  isLoading?: boolean;
}

export function InviteUserDialog({
  open,
  onOpenChange,
  onInvite,
  isLoading = false,
}: InviteUserDialogProps) {
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string>("");

  const handleInvite = () => {
    setError("");
    if (!username.trim()) {
      setError("Username is required");
      return;
    }
    if (!displayName.trim()) {
      setError("Display name is required");
      return;
    }
    if (!email.trim()) {
      setError("Email is required");
      return;
    }
    if (!password.trim()) {
      setError("Password is required");
      return;
    }
    if (password.length < 12) {
      setError("Password must be at least 12 characters");
      return;
    }
    onInvite?.(username, displayName, email, password);
  };

  const handleClose = () => {
    setUsername("");
    setDisplayName("");
    setEmail("");
    setPassword("");
    setError("");
    onOpenChange(false);
  };

  return (
    <Modal isOpen={open} onOpenChange={handleClose}>
      <ModalBackdrop isDismissable={!isLoading} isKeyboardDismissDisabled={isLoading} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Invite user</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <div>
              <Label htmlFor="invite-username" className="text-xs">
                Username
              </Label>
              <Input
                id="invite-username"
                autoFocus
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                placeholder="alice"
                className="mt-1"
                disabled={isLoading}
              />
            </div>

            <div>
              <Label htmlFor="invite-display-name" className="text-xs">
                Display name
              </Label>
              <Input
                id="invite-display-name"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder="Alice Operator"
                className="mt-1"
                disabled={isLoading}
              />
            </div>

            <div>
              <Label htmlFor="invite-email" className="text-xs">
                Email
              </Label>
              <Input
                id="invite-email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                placeholder="alice@example.com"
                className="mt-1"
                type="email"
                disabled={isLoading}
              />
            </div>

            <div>
              <Label htmlFor="invite-password" className="text-xs">
                Initial password
              </Label>
              <Input
                id="invite-password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="At least 12 characters"
                className="mt-1"
                type="password"
                disabled={isLoading}
              />
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
              onPress={handleInvite}
            >
              {isLoading ? "Inviting…" : "Invite user"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}
