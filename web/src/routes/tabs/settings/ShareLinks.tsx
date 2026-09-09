import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Button,
  Modal,
  ModalBackdrop,
  ModalContainer,
  ModalDialog,
  ModalHeader,
  ModalHeading,
  ModalBody,
  ModalFooter,
  AlertDialog,
  AlertDialogBackdrop,
  AlertDialogContainer,
  AlertDialogDialog,
  AlertDialogHeader,
  AlertDialogHeading,
  AlertDialogBody,
  AlertDialogFooter,
  AlertDialogIcon,
  Table,
  Select,
  ListBox,
  ListBoxItem,
  Label,
  Description,
  Switch,
  Chip,
} from "@heroui/react";
import { AlertCircle, Copy, Link2 } from "lucide-react";
import type { ShareLink } from "@/types";
import { Shares } from "@/lib/api";
import { errorText } from "@/lib/errors";

interface ShareLinksProps {
  name: string;
  ns?: string;
}

// Expiry options for the create dialog
const EXPIRY_OPTIONS = [
  { label: "24 hours", value: "24h" },
  { label: "7 days", value: "168h" },
  { label: "30 days", value: "720h" },
  { label: "90 days", value: "2160h" },
];

const EXPIRY_HELP_TEXT = "Maximum 90 days. You can revoke it earlier at any time.";

// Status badge coloring
function statusColor(status: string): "success" | "warning" | "danger" | "default" {
  if (status === "Active") return "success";
  if (status === "Expired") return "warning";
  if (status === "Revoked") return "danger";
  return "default";
}

// Determine link status based on expiry
function getLinkStatus(expiresAt: string): string {
  const expiry = new Date(expiresAt);
  const now = new Date();
  if (expiry <= now) return "Expired";
  return "Active";
}

// Format date for display (e.g., "Jul 28, 2026")
function formatDate(dateStr: string): string {
  const date = new Date(dateStr);
  return date.toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
  });
}

// CreateDialog: Modal for entering link details
interface CreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  serverName: string;
  ns?: string;
  onLinkCreated?: (link: ShareLink) => void;
}

function CreateDialog({
  open,
  onOpenChange,
  serverName,
  ns,
  onLinkCreated,
}: CreateDialogProps) {
  const [expiry, setExpiry] = useState("168h");
  const [canStart, setCanStart] = useState(false);

  const create = useMutation({
    mutationFn: () =>
      Shares.create(
        serverName,
        {
          expiresIn: expiry,
          canStart,
        },
        ns,
      ),
    onSuccess: (link) => {
      onOpenChange(false);
      onLinkCreated?.(link);
    },
  });

  // Reset state when dialog opens
  const [prevOpen, setPrevOpen] = useState(open);
  if (open !== prevOpen) {
    setPrevOpen(open);
    if (open) {
      setExpiry("168h");
      setCanStart(false);
      create.reset();
    }
  }

  return (
    <Modal isOpen={open} onOpenChange={onOpenChange}>
      <ModalBackdrop
        isDismissable={!create.isPending}
        isKeyboardDismissDisabled={create.isPending}
      />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Create share link for {serverName}</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <Description className="text-sm text-muted">
              Anyone with this link can check the server&apos;s status and connection address
              without signing in.
            </Description>

            <div>
              <Select value={expiry} onChange={(key) => setExpiry(String(key))} className="mt-1">
                <Label className="text-xs">Expires in</Label>
                <Select.Trigger>
                  <Select.Value />
                  <Select.Indicator />
                </Select.Trigger>
                <Select.Popover>
                  <ListBox>
                    {EXPIRY_OPTIONS.map((opt) => (
                      <ListBoxItem key={opt.value} id={opt.value}>
                        {opt.label}
                      </ListBoxItem>
                    ))}
                  </ListBox>
                </Select.Popover>
              </Select>
              <Description className="mt-1 text-xs text-muted">
                {EXPIRY_HELP_TEXT}
              </Description>
            </div>

            <div className="flex items-center gap-3">
              <Switch
                isSelected={canStart}
                onChange={setCanStart}
                aria-label="Allow starting the server"
              >
                <Switch.Content>
                  <Switch.Control>
                    <Switch.Thumb />
                  </Switch.Control>
                </Switch.Content>
              </Switch>
              <div className="flex-1">
                <Label className="text-sm">Allow starting the server</Label>
                <Description className="text-xs text-muted">
                  The link holder can wake {serverName} from asleep. Without this, they can
                  only view its status.
                </Description>
              </div>
            </div>

            {create.isError && (
              <div className="rounded-lg bg-danger/10 p-3 text-sm text-danger">
                {errorText(create.error, "Failed to create link")}
              </div>
            )}
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              onPress={() => onOpenChange(false)}
              isDisabled={create.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              isDisabled={create.isPending}
              onPress={() => create.mutate()}
            >
              {create.isPending ? "Creating…" : "Create link"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}

// CreatedDialog: Modal showing the generated link with copy button
interface CreatedDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  link: ShareLink | null;
}

function CreatedDialog({ open, onOpenChange, link }: CreatedDialogProps) {
  const [copied, setCopied] = useState(false);

  if (!link?.token) return null;

  const url = `${window.location.origin}/share/${link.token}`;

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(url);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Silently fail if clipboard is not available
    }
  };

  return (
    <Modal isOpen={open} onOpenChange={onOpenChange}>
      <ModalBackdrop isDismissable>
        <ModalContainer>
          <ModalDialog>
            <ModalHeader>
              <ModalHeading>Share link created</ModalHeading>
            </ModalHeader>

            <ModalBody className="gap-4">
              <Description className="text-sm text-foreground">
                Send this to your friend. It works without a Gameplane account.
              </Description>

              <div className="rounded-lg border-2 border-warning bg-warning/10 p-3">
                <div className="flex items-start gap-2">
                  <AlertCircle className="h-5 w-5 shrink-0 text-warning" />
                  <div className="text-sm text-warning">
                    <strong>You will not see this link again</strong>
                    <p className="mt-1">
                      Gameplane stores only a one-way hash of the token, so it can&apos;t be shown
                      or recovered after you close this dialog. Copy it now.
                    </p>
                  </div>
                </div>
              </div>

              <div className="rounded-lg bg-surface p-3">
                <div className="break-all font-mono text-sm text-foreground">{url}</div>
                <Button
                  size="sm"
                  variant="secondary"
                  className="mt-3"
                  onPress={handleCopy}
                >
                  <Copy className="h-4 w-4" />
                  {copied ? "Copied!" : "Copy link"}
                </Button>
              </div>
            </ModalBody>

            <ModalFooter className="flex items-center justify-end gap-2">
              <Button
                size="sm"
                variant="primary"
                onPress={() => onOpenChange(false)}
              >
                Done
              </Button>
            </ModalFooter>
          </ModalDialog>
        </ModalContainer>
      </ModalBackdrop>
    </Modal>
  );
}

// RevokeDialog: AlertDialog for confirming revocation
interface RevokeDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  linkId: string | null;
  serverName: string;
  ns?: string;
  onRevoked?: () => void;
}

function RevokeDialog({
  open,
  onOpenChange,
  linkId,
  serverName,
  ns,
  onRevoked,
}: RevokeDialogProps) {
  const revoke = useMutation({
    mutationFn: () => (linkId ? Shares.revoke(serverName, linkId, ns) : Promise.reject()),
    onSuccess: () => {
      onOpenChange(false);
      onRevoked?.();
    },
  });

  return (
    <AlertDialog isOpen={open} onOpenChange={onOpenChange}>
      <AlertDialogBackdrop
        isDismissable={!revoke.isPending}
        isKeyboardDismissDisabled={revoke.isPending}
      />
      <AlertDialogContainer>
        <AlertDialogDialog>
          <AlertDialogHeader>
            <div className="flex items-start gap-4">
              <AlertDialogIcon status="danger" className="shrink-0">
                <AlertCircle className="h-5 w-5" />
              </AlertDialogIcon>
              <AlertDialogHeading>Revoke this share link?</AlertDialogHeading>
            </div>
          </AlertDialogHeader>

          <AlertDialogBody>
            <div className="space-y-4">
              <Description className="text-sm text-foreground">
                Anyone using this link will immediately lose access to {serverName}&apos;s status
                page. This action cannot be undone.
              </Description>
              {revoke.isError && (
                <div className="rounded-lg bg-danger/10 p-3 text-sm text-danger">
                  {errorText(revoke.error, "Failed to revoke link")}
                </div>
              )}
            </div>
          </AlertDialogBody>

          <AlertDialogFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              isDisabled={revoke.isPending}
              onPress={() => onOpenChange(false)}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="danger"
              isDisabled={revoke.isPending}
              onPress={() => revoke.mutate()}
            >
              {revoke.isPending ? "Revoking…" : "Revoke link"}
            </Button>
          </AlertDialogFooter>
        </AlertDialogDialog>
      </AlertDialogContainer>
    </AlertDialog>
  );
}

// Main component
export function ShareLinksSection({ name, ns }: ShareLinksProps) {
  const qc = useQueryClient();
  const [createOpen, setCreateOpen] = useState(false);
  const [createdLink, setCreatedLink] = useState<ShareLink | null>(null);
  const [createdOpen, setCreatedOpen] = useState(false);
  const [revokeOpen, setRevokeOpen] = useState(false);
  const [revokeId, setRevokeId] = useState<string | null>(null);

  const { data: links, isLoading } = useQuery({
    queryKey: ["sharelinks", name, ns],
    queryFn: () => Shares.list(name, ns),
    enabled: !!name,
  });

  const handleLinkCreated = (link: ShareLink) => {
    setCreatedLink(link);
    setCreatedOpen(true);
    void qc.invalidateQueries({ queryKey: ["sharelinks", name, ns] });
  };

  const handleRevoked = () => {
    void qc.invalidateQueries({ queryKey: ["sharelinks", name, ns] });
  };

  const empty = !isLoading && (!links || links.length === 0);

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between">
        <div>
          <h3 className="text-base font-medium">Share links</h3>
          <p className="mt-1 text-sm text-muted">
            Let people without a Gameplane account check this server&apos;s status and connection
            address. Only the owner can create or revoke links.
          </p>
        </div>
        <Button variant="primary" size="sm" onPress={() => setCreateOpen(true)}>
          <Link2 className="h-4 w-4" />
          Create link
        </Button>
      </div>

      {empty ? (
        <div className="rounded-lg border border-dashed border-divider p-8 text-center">
          <Link2 className="mx-auto h-12 w-12 text-muted" />
          <h4 className="mt-3 font-medium">No share links yet</h4>
          <p className="mt-1 text-sm text-muted">
            Create a link so a friend without a Gameplane account can check this server&apos;s
            status and connect — without giving them access to the dashboard.
          </p>
          <Button
            variant="primary"
            size="sm"
            className="mt-4"
            onPress={() => setCreateOpen(true)}
          >
            <Link2 className="h-4 w-4" />
            Create link
          </Button>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-divider">
          <Table.Root>
            <Table.Content aria-label="Share links">
              <Table.Header>
                <Table.Column isRowHeader>Created</Table.Column>
                <Table.Column>Expires</Table.Column>
                <Table.Column>Can start</Table.Column>
                <Table.Column>Status</Table.Column>
                <Table.Column>Actions</Table.Column>
              </Table.Header>
              <Table.Body>
                {links?.map((link) => (
                  <Table.Row key={link.id}>
                    <Table.Cell>{formatDate(link.createdAt)}</Table.Cell>
                    <Table.Cell>{formatDate(link.expiresAt)}</Table.Cell>
                    <Table.Cell>
                      <Chip
                        variant="soft"
                        size="sm"
                        className={link.canStart ? "text-success" : "text-foreground"}
                      >
                        {link.canStart ? "Can start" : "View only"}
                      </Chip>
                    </Table.Cell>
                    <Table.Cell>
                      <Chip
                        variant="soft"
                        size="sm"
                        color={statusColor(getLinkStatus(link.expiresAt))}
                      >
                        {getLinkStatus(link.expiresAt)}
                      </Chip>
                    </Table.Cell>
                    <Table.Cell>
                      <Button
                        variant="danger-soft"
                        size="sm"
                        onPress={() => {
                          setRevokeId(link.id);
                          setRevokeOpen(true);
                        }}
                      >
                        Revoke
                      </Button>
                    </Table.Cell>
                  </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.Root>
        </div>
      )}

      <CreateDialog
        open={createOpen}
        onOpenChange={setCreateOpen}
        serverName={name}
        ns={ns}
        onLinkCreated={handleLinkCreated}
      />

      <CreatedDialog open={createdOpen} onOpenChange={setCreatedOpen} link={createdLink} />

      <RevokeDialog
        open={revokeOpen}
        onOpenChange={setRevokeOpen}
        linkId={revokeId}
        serverName={name}
        ns={ns}
        onRevoked={handleRevoked}
      />
    </div>
  );
}
