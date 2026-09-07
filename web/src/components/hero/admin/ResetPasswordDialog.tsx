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
} from "@heroui/react";

export interface ResetPasswordDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  username?: string;
  onReset?: (password: string) => void;
  isLoading?: boolean;
}

export function ResetPasswordDialog({
  open,
  onOpenChange,
  username = "",
  onReset,
  isLoading = false,
}: ResetPasswordDialogProps) {
  const [password, setPassword] = useState("");
  const [validationError, setValidationError] = useState<string>("");

  const handleReset = () => {
    setValidationError("");
    if (!password.trim()) {
      setValidationError("Password is required");
      return;
    }
    if (password.length < 12) {
      setValidationError("Must be at least 12 characters.");
      return;
    }
    onReset?.(password);
  };

  const handleClose = () => {
    setPassword("");
    setValidationError("");
    onOpenChange(false);
  };

  return (
    <Modal isOpen={open} onOpenChange={handleClose}>
      <ModalBackdrop isDismissable={!isLoading} isKeyboardDismissDisabled={isLoading} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Reset password for {username}</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <div>
              <Label htmlFor="reset-password" className="text-xs">
                New password
              </Label>
              <Input
                id="reset-password"
                autoFocus
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="At least 12 characters"
                className="mt-1"
                type="password"
                disabled={isLoading}
              />
              {validationError && (
                <p className="mt-1 text-xs text-danger" role="alert">
                  {validationError}
                </p>
              )}
            </div>
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
              onPress={handleReset}
            >
              {isLoading ? "Resetting…" : "Reset password"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}
