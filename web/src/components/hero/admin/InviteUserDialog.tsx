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
  Select,
  ListBox,
  ListBoxItem,
} from "@heroui/react";

const MIN_PASSWORD_LEN = 12;

export interface InviteUserDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onInvite?: (
    username: string,
    displayName: string,
    email: string,
    password: string,
    role?: string,
  ) => void;
  isLoading?: boolean;
  /**
   * Role names offered in a "Role" select. When omitted (or empty), no role
   * field is rendered and `onInvite` is called with its original 4-arg
   * signature — this keeps every pre-existing caller/test unaffected.
   */
  roles?: string[];
  /**
   * When true, display name/email/password are optional (matching the
   * "create a local account, leave password blank for an OIDC invite"
   * flow) — only username is required, and password is validated for
   * minimum length only when non-empty. Defaults to false, preserving the
   * dialog's original all-fields-required behavior.
   */
  contactFieldsOptional?: boolean;
  /**
   * When true, the submit button is preemptively disabled while the
   * username is empty or the password is present-but-too-short, instead of
   * relying on a click to reveal the validation error. Defaults to false,
   * preserving the original click-to-validate behavior.
   */
  disableSubmitUntilValid?: boolean;
  /** Error text from an external (API) failure, shown alongside client validation. */
  apiError?: string;
  submitLabel?: string;
}

export function InviteUserDialog({
  open,
  onOpenChange,
  onInvite,
  isLoading = false,
  roles,
  contactFieldsOptional = false,
  disableSubmitUntilValid = false,
  apiError,
  submitLabel = "Invite user",
}: InviteUserDialogProps) {
  const hasRoles = !!roles && roles.length > 0;
  const [username, setUsername] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [role, setRole] = useState(roles?.includes("viewer") ? "viewer" : (roles?.[0] ?? "viewer"));
  const [error, setError] = useState<string>("");

  const passwordTooShort = password.length > 0 && password.length < MIN_PASSWORD_LEN;
  const liveError = disableSubmitUntilValid && passwordTooShort ? `At least ${MIN_PASSWORD_LEN} characters.` : "";
  const submitDisabled =
    isLoading || (disableSubmitUntilValid && (!username.trim() || passwordTooShort));

  const handleInvite = () => {
    setError("");
    if (!username.trim()) {
      setError("Username is required");
      return;
    }
    if (!contactFieldsOptional) {
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
    }
    if (passwordTooShort) {
      setError(
        contactFieldsOptional
          ? `At least ${MIN_PASSWORD_LEN} characters.`
          : `Password must be at least ${MIN_PASSWORD_LEN} characters`,
      );
      return;
    }
    if (hasRoles) {
      onInvite?.(username, displayName, email, password, role);
    } else {
      onInvite?.(username, displayName, email, password);
    }
  };

  const handleClose = () => {
    setUsername("");
    setDisplayName("");
    setEmail("");
    setPassword("");
    setRole(roles?.includes("viewer") ? "viewer" : (roles?.[0] ?? "viewer"));
    setError("");
    onOpenChange(false);
  };

  const displayError = error || liveError || apiError;

  return (
    <Modal isOpen={open} onOpenChange={handleClose}>
      <ModalBackdrop isDismissable={!isLoading} isKeyboardDismissDisabled={isLoading}>
        <ModalContainer>
          <ModalDialog>
            <ModalHeader>
              <ModalHeading>Invite user</ModalHeading>
            </ModalHeader>

            <ModalBody className="gap-4">
            {contactFieldsOptional && (
              <Description>
                Create a local account. Leave password blank to send an OIDC invite later.
              </Description>
            )}

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

            <div className="grid grid-cols-2 gap-3">
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
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label htmlFor="invite-password" className="text-xs">
                  Initial password
                </Label>
                <Input
                  id="invite-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder={`At least ${MIN_PASSWORD_LEN} characters`}
                  className="mt-1"
                  type="password"
                  disabled={isLoading}
                />
              </div>

              {hasRoles && (
                <div className="space-y-1">
                  <Select value={role} onChange={(v) => setRole(String(v))} isDisabled={isLoading}>
                    <Label htmlFor="invite-role" className="text-xs">
                      Role
                    </Label>
                    <Select.Trigger
                      id="invite-role"
                      className="w-full rounded border border-border bg-surface px-3 py-2 text-sm hover:bg-surface/80"
                    >
                      <Select.Value />
                      <Select.Indicator className="ml-auto h-4 w-4" />
                    </Select.Trigger>
                    <Select.Popover className="rounded border border-border">
                      <ListBox className="p-0" aria-label="Role">
                        {roles?.map((r) => (
                          <ListBoxItem key={r} id={r}>
                            {r}
                          </ListBoxItem>
                        ))}
                      </ListBox>
                    </Select.Popover>
                  </Select>
                </div>
              )}
            </div>

            {displayError && (<p className="text-xs text-danger" role="alert">{displayError}</p>)}
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button variant="secondary" size="sm" onPress={handleClose} isDisabled={isLoading}>
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              isDisabled={submitDisabled}
              onPress={handleInvite}
            >
              {isLoading ? "Inviting…" : submitLabel}
            </Button>
            </ModalFooter>
          </ModalDialog>
        </ModalContainer>
      </ModalBackdrop>
    </Modal>
  );
}
