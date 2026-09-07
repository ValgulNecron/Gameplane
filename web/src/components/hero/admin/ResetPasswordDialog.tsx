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
  Description,
} from "@heroui/react";

const MIN_PASSWORD_LEN = 12;

export interface ResetPasswordDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  username?: string;
  onReset?: (password: string) => void;
  isLoading?: boolean;
  /**
   * When true, the submit button is preemptively disabled while the
   * password is shorter than the minimum length, instead of relying on a
   * click to reveal the validation error. Defaults to false, preserving
   * the original click-to-validate behavior.
   */
  disableSubmitUntilValid?: boolean;
  /** Error text from an external (API) failure, shown alongside client validation. */
  apiError?: string;
  submitLabel?: string;
}

export function ResetPasswordDialog({
  open,
  onOpenChange,
  username = "",
  onReset,
  isLoading = false,
  disableSubmitUntilValid = false,
  apiError,
  submitLabel = "Reset password",
}: ResetPasswordDialogProps) {
  const [password, setPassword] = useState("");
  const [validationError, setValidationError] = useState<string>("");

  const submitDisabled =
    isLoading || (disableSubmitUntilValid && password.length < MIN_PASSWORD_LEN);

  const liveError = disableSubmitUntilValid && password.length > 0 && password.length < MIN_PASSWORD_LEN ? `At least ${MIN_PASSWORD_LEN} characters.` : "";

  const handleReset = () => {
    setValidationError("");
    if (!password.trim()) {
      setValidationError("Password is required");
      return;
    }
    if (password.length < MIN_PASSWORD_LEN) {
      setValidationError(`Must be at least ${MIN_PASSWORD_LEN} characters.`);
      return;
    }
    onReset?.(password);
  };

  const handleClose = () => {
    setPassword("");
    setValidationError("");
    onOpenChange(false);
  };

  const displayError = validationError || liveError || apiError;

  return (
    <Modal isOpen={open} onOpenChange={handleClose}>
      <ModalBackdrop isDismissable={!isLoading} isKeyboardDismissDisabled={isLoading} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Reset password for {username}</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <Description>They will need to sign in again with the new password.</Description>
            <div>
              <Label htmlFor="reset-password" className="text-xs">
                New password
              </Label>
              <Input
                id="reset-password"
                autoFocus
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder={`At least ${MIN_PASSWORD_LEN} characters`}
                className="mt-1"
                type="password"
                disabled={isLoading}
              />
              {displayError && (
                <p className="mt-1 text-xs text-danger" role="alert">{displayError}</p>
              )}
            </div>
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button variant="secondary" size="sm" onPress={handleClose} isDisabled={isLoading}>
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              isDisabled={submitDisabled}
              onPress={handleReset}
            >
              {isLoading ? "Resetting…" : submitLabel}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}
