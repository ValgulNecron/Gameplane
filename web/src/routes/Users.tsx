import { useMemo, useState } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import {
  Button,
  Card,
  Input,
  Tabs,
  Tab as TabComponent,
  Table,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownPopover,
  DropdownSection,
  DropdownItem,
  Modal,
  ModalDialog,
  ModalHeader,
  ModalHeading,
  ModalBody,
  ModalFooter,
  AlertDialog,
  AlertDialogBackdrop,
  AlertDialogContainer,
  Alert,
  Separator,
} from "@heroui/react";
import {
  KeyRound,
  MoreHorizontal,
  Pencil,
  Plus,
  ScrollText,
  Search,
  Trash2,
} from "lucide-react";
import { PageHeader } from "@/components/hero/PageHeader";
import { RoleEditorModal } from "@/components/hero/RoleEditorModal";
import { InviteUserDialog } from "@/components/hero/admin/InviteUserDialog";
import { EditUserDialog } from "@/components/hero/admin/EditUserDialog";
import { ResetPasswordDialog } from "@/components/hero/admin/ResetPasswordDialog";
import { APIError } from "@/lib/api";
import { useMe, can } from "@/lib/auth";
import {
  Users as UsersAPI,
  Roles as RolesAPI,
  type UserUpdate,
} from "@/lib/endpoints";
import { formatRelative } from "@/lib/utils";
import type { ExtendedUser, Role, RoleBinding } from "@/types";

type TabKey = "users" | "roles" | "service" | "idp";

const roleColor: Record<string, string> = {
  admin: "bg-primary/15 text-primary",
  operator: "bg-violet/15 text-violet",
  viewer: "bg-muted/20 text-muted",
};

function apiErrorText(error: unknown): string | undefined {
  if (!error) return undefined;
  return error instanceof APIError
    ? error.body || `Request failed (${error.status})`
    : (error as Error).message;
}

function roleGrantsUserManagement(roles: Role[], name: string): boolean {
  const r = roles.find((x) => x.name === name);
  return !!r && (r.permissions.includes("*") || r.permissions.includes("users:manage"));
}

function useRolesQuery() {
  return useQuery({ queryKey: ["roles"], queryFn: () => RolesAPI.list() });
}

export function UsersPage() {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const canAudit = can(me, "audit:read");
  const [tab, setTab] = useState<TabKey>("users");
  const [q, setQ] = useState("");
  const [inviting, setInviting] = useState(false);
  const [editing, setEditing] = useState<ExtendedUser | null>(null);
  const [resetting, setResetting] = useState<ExtendedUser | null>(null);
  const [deleting, setDeleting] = useState<ExtendedUser | null>(null);

  const { data: users = [], error } = useQuery({
    queryKey: ["users"],
    queryFn: () => UsersAPI.list(),
  });
  const { data: roles = [] } = useRolesQuery();

  const counts = useMemo(
    () => ({
      users: users.length,
      roles: roles.length,
    }),
    [users, roles],
  );

  const visible = users.filter((u) => {
    if (q) {
      const s = q.toLowerCase();
      return (
        u.username.toLowerCase().includes(s) ||
        (u.email ?? "").toLowerCase().includes(s) ||
        (u.displayName ?? "").toLowerCase().includes(s)
      );
    }
    return true;
  });

  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ["users"] });
  };

  const create = useMutation({
    mutationFn: UsersAPI.create,
    onSuccess: () => {
      invalidate();
      setInviting(false);
    },
  });
  const save = useMutation({
    mutationFn: ({ id, body }: { id: number; body: UserUpdate }) => UsersAPI.update(id, body),
    onSuccess: () => {
      invalidate();
      setEditing(null);
    },
  });
  const reset = useMutation({
    mutationFn: ({ id, password }: { id: number; password: string }) =>
      UsersAPI.resetPassword(id, password),
    onSuccess: () => setResetting(null),
  });

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Users & RBAC"
        description="Manage access to the Gameplane control plane."
        actions={
          <div className="flex items-center gap-2">
            {canAudit && (
              <Link
                to="/admin/audit"
                className="inline-flex h-9 items-center gap-1.5 rounded-md border border-border px-3 text-sm font-medium text-foreground transition-colors hover:bg-surface"
              >
                <ScrollText className="h-4 w-4" /> Audit log
              </Link>
            )}
            <Button onPress={() => setInviting(true)} variant="primary">
              <Plus className="h-4 w-4" /> Invite user
            </Button>
          </div>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <Tabs
          selectedKey={tab}
          onSelectionChange={(key) => setTab(key as TabKey)}
          variant="secondary"
        >
          <Tabs.List>
            <TabComponent id="users">
              <div className="flex items-center gap-2">
                <span>Users</span>
                <span className="text-xs text-foreground/60">({counts.users})</span>
              </div>
            </TabComponent>
            <TabComponent id="roles">
              <div className="flex items-center gap-2">
                <span>Roles</span>
                <span className="text-xs text-foreground/60">({counts.roles})</span>
              </div>
            </TabComponent>
            <TabComponent id="service">Service accounts</TabComponent>
            <TabComponent id="idp">Identity providers</TabComponent>
          </Tabs.List>

          <Tabs.Panel id="users">
            {error instanceof APIError && (
              <Alert status="danger">
                <Alert.Indicator />
                <Alert.Title>Failed to load users.</Alert.Title>
                {error.body && <Alert.Description>{error.body}</Alert.Description>}
              </Alert>
            )}

            <Table.Root>
              <Table.ScrollContainer>
                <Table.Content aria-label="Users list">
                  <Table.Header>
                    <Table.Column id="user" isRowHeader>User</Table.Column>
                    <Table.Column id="role">Role</Table.Column>
                    <Table.Column id="provider">Provider</Table.Column>
                    <Table.Column id="created">Created</Table.Column>
                    <Table.Column id="actions" className="text-right">Actions</Table.Column>
                  </Table.Header>
                  <Table.Body
                    renderEmptyState={() => <span>No entries.</span>}
                  >
                    {visible.map((u) => (
                  <Table.Row key={u.id}>
                    <Table.Cell>
                      <div className="flex items-center gap-3">
                        <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/20 font-mono text-xs text-primary">
                          {(u.displayName || u.username).slice(0, 2).toUpperCase()}
                        </div>
                        <div className="min-w-0">
                          <div className="truncate font-mono text-sm text-foreground">
                            {u.username}
                            {me && me.id === u.id && (
                              <span className="ml-2 rounded bg-foreground/10 px-1.5 py-0.5 text-[10px] text-foreground/70">
                                you
                              </span>
                            )}
                          </div>
                          <div className="truncate text-[11px] text-foreground/60">
                            {u.displayName || u.email || "—"}
                          </div>
                          {u.email && u.displayName && u.displayName !== u.email && (
                            <div className="truncate text-[11px] text-foreground/60">{u.email}</div>
                          )}
                        </div>
                      </div>
                    </Table.Cell>
                    <Table.Cell>
                      <span className={`rounded px-2 py-0.5 text-[10px] font-mono uppercase ${roleColor[u.role] ?? "bg-muted/20 text-muted"}`}>
                        {u.role}
                      </span>
                    </Table.Cell>
                    <Table.Cell>
                      <span className="rounded px-2 py-0.5 text-[10px] uppercase text-foreground/60 ring-1 ring-border">
                        {u.provider === "oidc" ? "OIDC" : u.provider === "pending" ? "Pending" : "Local"}
                      </span>
                    </Table.Cell>
                    <Table.Cell className="text-foreground/60">{formatRelative(u.createdAt)}</Table.Cell>
                    <Table.Cell>
                      <Dropdown>
                        <DropdownTrigger
                          className="inline-flex h-9 w-9 items-center justify-center rounded-md hover:bg-default-100 text-foreground/60 hover:text-foreground"
                          aria-label={`Actions for ${u.username}`}
                        >
                          <>
                            <MoreHorizontal className="h-4 w-4" />
                          </>
                        </DropdownTrigger>
                        <DropdownPopover>
                          <DropdownMenu aria-label={`Actions for ${u.username}`}>
                            <DropdownItem
                              key="edit"
                              onPress={() => setEditing(u)}
                            >
                              <div className="flex items-center gap-2">
                                <Pencil className="h-4 w-4" />
                                Edit user
                              </div>
                            </DropdownItem>
                            <DropdownItem
                              key="reset"
                              isDisabled={u.provider === "oidc"}
                              onPress={() => setResetting(u)}
                              aria-label={u.provider === "oidc" ? "Reset password: Account is OIDC-managed" : "Reset password"}
                            >
                              <div className="flex items-center gap-2" title={u.provider === "oidc" ? "Account is OIDC-managed" : undefined}>
                                <KeyRound className="h-4 w-4" />
                                Reset password
                              </div>
                            </DropdownItem>
                            <DropdownSection>
                              <DropdownItem
                                key="delete"
                                variant="danger"
                                isDisabled={Boolean(me && me.id === u.id)}
                                onPress={() => setDeleting(u)}
                              >
                                <div className="flex items-center gap-2">
                                  <Trash2 className="h-4 w-4" />
                                  Delete user
                                </div>
                              </DropdownItem>
                            </DropdownSection>
                          </DropdownMenu>
                        </DropdownPopover>
                      </Dropdown>
                    </Table.Cell>
                  </Table.Row>
                    ))}
                  </Table.Body>
                </Table.Content>
              </Table.ScrollContainer>
            </Table.Root>
          </Tabs.Panel>

          <Tabs.Panel id="roles">
            <RolesTab />
          </Tabs.Panel>

          <Tabs.Panel id="service">
            <ServiceAccountsTab />
          </Tabs.Panel>

          <Tabs.Panel id="idp">
            <IdpTab />
          </Tabs.Panel>
        </Tabs>
        <div className="relative ml-auto w-64">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-foreground/60" />
          <Input
            className="pl-9"
            placeholder={tab === "users" ? "Search users…" : "Search…"}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            aria-label="Search users"
          />
        </div>
      </div>

      <InviteUserDialog
        open={inviting}
        onOpenChange={(open) => {
          if (!open) {
            setInviting(false);
            create.reset();
          }
        }}
        roles={roles.map((r) => r.name)}
        contactFieldsOptional
        disableSubmitUntilValid
        isLoading={create.isPending}
        apiError={apiErrorText(create.error)}
        submitLabel="Create user"
        onInvite={(username, displayName, email, password, role) => {
          create.mutate({
            username,
            displayName: displayName || undefined,
            email: email || undefined,
            password: password || undefined,
            role: role ?? "viewer",
          });
        }}
      />
      {editing && (
        <EditUserDialog
          key={editing.id}
          open
          onOpenChange={(open) => {
            if (!open) {
              setEditing(null);
              save.reset();
            }
          }}
          username={editing.username}
          displayName={editing.displayName ?? ""}
          email={editing.email ?? ""}
          role={editing.role}
          roles={roles.map((r) => r.name)}
          isMe={!!me && me.id === editing.id}
          roleGrantsUserManagement={(r) => roleGrantsUserManagement(roles, r)}
          isLoading={save.isPending}
          apiError={apiErrorText(save.error)}
          contactFieldsOptional
          extraContent={<NamespaceGrants userId={editing.id} roles={roles} />}
          onSave={(displayName, email, role) => {
            const dirty: UserUpdate = {};
            if (displayName !== (editing.displayName ?? "")) dirty.displayName = displayName;
            if (email !== (editing.email ?? "")) dirty.email = email;
            if (role !== editing.role) dirty.role = role;
            if (Object.keys(dirty).length === 0) return;
            save.mutate({ id: editing.id, body: dirty });
          }}
        />
      )}
      {resetting && (
        <ResetPasswordDialog
          open
          onOpenChange={(open) => {
            if (!open) {
              setResetting(null);
              reset.reset();
            }
          }}
          username={resetting.username}
          disableSubmitUntilValid
          isLoading={reset.isPending}
          apiError={apiErrorText(reset.error)}
          submitLabel="Set new password"
          onReset={(password) => reset.mutate({ id: resetting.id, password })}
        />
      )}
      {deleting && (
        <DeleteUserDialog
          user={deleting}
          isMe={!!me && me.id === deleting.id}
          onClose={() => setDeleting(null)}
          onDeleted={() => {
            invalidate();
            setDeleting(null);
          }}
        />
      )}
    </div>
  );
}

function ErrorLine({ error }: { error: unknown }) {
  if (!error) return null;
  const text =
    error instanceof APIError
      ? error.body || `Request failed (${error.status})`
      : (error as Error).message;
  return (
    <p role="alert" className="pt-2 text-xs text-danger">
      {text}
    </p>
  );
}

function DeleteUserDialog({
  user,
  isMe,
  onClose,
  onDeleted,
}: {
  user: ExtendedUser;
  isMe: boolean;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const remove = useMutation({
    mutationFn: () => UsersAPI.remove(user.id),
    onSuccess: onDeleted,
  });

  return (
    <AlertDialog isOpen onOpenChange={(open) => !open && onClose()}>
      <AlertDialogBackdrop isDismissable={!remove.isPending} isKeyboardDismissDisabled={remove.isPending}>
        <AlertDialogContainer>
          <Modal>
            <ModalDialog role="alertdialog" className="max-w-md">
              <ModalHeader>
                <ModalHeading>Delete {user.username}?</ModalHeading>
              </ModalHeader>
              <ModalBody>
                {isMe ? (
                  <p className="text-sm text-danger">You can&apos;t delete your own account.</p>
                ) : (
                  <div className="space-y-2">
                    <p className="text-sm">
                      Their sessions will be revoked and they&apos;ll lose access immediately. This cannot be undone.
                    </p>
                    <ErrorLine error={remove.error} />
                  </div>
                )}
              </ModalBody>
              <ModalFooter className="flex items-center justify-end gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  onPress={onClose}
                  isDisabled={remove.isPending}
                >
                  Cancel
                </Button>
                <Button
                  size="sm"
                  variant="danger"
                  isDisabled={isMe || remove.isPending}
                  onPress={() => remove.mutate()}
                >
                  {isMe ? "Cannot delete" : "Delete user"}
                </Button>
              </ModalFooter>
            </ModalDialog>
          </Modal>
        </AlertDialogContainer>
      </AlertDialogBackdrop>
    </AlertDialog>
  );
}

function RolesTab() {
  const qc = useQueryClient();
  const { data: me } = useMe();
  const { data: roles = [] } = useRolesQuery();
  const { data: catalog } = useQuery({
    queryKey: ["permission-catalog"],
    queryFn: () => RolesAPI.catalog(),
  });
  const canManage = can(me, "roles:manage");
  const [editing, setEditing] = useState<Role | null>(null);
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<Role | null>(null);

  const refresh = () => void qc.invalidateQueries({ queryKey: ["roles"] });
  const groups = catalog?.groups ?? [];

  return (
    <div className="space-y-4">
      <div className="flex justify-end">
        {canManage && (
          <Button onPress={() => setCreating(true)} variant="primary">
            <Plus className="h-4 w-4" /> New role
          </Button>
        )}
      </div>
      <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
        {roles.map((r) => (
          <Card key={r.name} className="flex flex-col gap-2 p-4">
            <div className="flex items-center justify-between gap-2">
              <span className="font-mono text-sm">{r.name}</span>
              {r.builtin && (
                <span className="rounded bg-foreground/10 px-1.5 py-0.5 text-[10px] uppercase text-foreground/70">
                  built-in
                </span>
              )}
            </div>
            <p className="min-h-8 text-xs text-foreground/60">{r.description || "—"}</p>
            <div className="text-[11px] text-foreground/60">
              {r.permissions.includes("*")
                ? "all permissions"
                : `${r.permissions.length} permission${r.permissions.length === 1 ? "" : "s"}`}
            </div>
            {canManage && (
              <div className="flex gap-2 pt-1">
                {r.name !== "admin" && (
                  <Button variant="ghost" className="h-7 px-2 text-xs" onPress={() => setEditing(r)}>
                    <Pencil className="h-3.5 w-3.5" /> Edit
                  </Button>
                )}
                {!r.builtin && (
                  <Button
                    variant="ghost"
                    className="h-7 px-2 text-xs text-danger"
                    onPress={() => setDeleting(r)}
                  >
                    <Trash2 className="h-3.5 w-3.5" /> Delete
                  </Button>
                )}
              </div>
            )}
          </Card>
        ))}
      </div>

      {(creating || editing) && (
        <RoleEditorModal
          open
          onOpenChange={(open) => {
            if (!open) {
              setCreating(false);
              setEditing(null);
            }
          }}
          role={editing}
          groups={groups}
          onSaved={() => {
            refresh();
            setCreating(false);
            setEditing(null);
          }}
        />
      )}
      {deleting && (
        <DeleteRoleDialog
          role={deleting}
          onClose={() => setDeleting(null)}
          onDeleted={() => {
            refresh();
            setDeleting(null);
          }}
        />
      )}
    </div>
  );
}

function DeleteRoleDialog({
  role,
  onClose,
  onDeleted,
}: {
  role: Role;
  onClose: () => void;
  onDeleted: () => void;
}) {
  const remove = useMutation({
    mutationFn: () => RolesAPI.remove(role.name),
    onSuccess: onDeleted,
  });

  return (
    <AlertDialog isOpen onOpenChange={(open) => !open && onClose()}>
      <AlertDialogBackdrop isDismissable={!remove.isPending} isKeyboardDismissDisabled={remove.isPending}>
        <AlertDialogContainer>
          <Modal>
            <ModalDialog role="alertdialog" className="max-w-md">
              <ModalHeader>
                <ModalHeading>Delete role {role.name}?</ModalHeading>
              </ModalHeader>
              <ModalBody>
                <div className="space-y-2">
                  <p className="text-sm">
                    This can&apos;t be undone. Roles assigned to a user can&apos;t be deleted.
                  </p>
                  <ErrorLine error={remove.error} />
                </div>
              </ModalBody>
              <ModalFooter className="flex items-center justify-end gap-2">
                <Button
                  variant="secondary"
                  size="sm"
                  onPress={onClose}
                  isDisabled={remove.isPending}
                >
                  Cancel
                </Button>
                <Button
                  size="sm"
                  variant="danger"
                  isDisabled={remove.isPending}
                  onPress={() => remove.mutate()}
                >
                  Delete role
                </Button>
              </ModalFooter>
            </ModalDialog>
          </Modal>
        </AlertDialogContainer>
      </AlertDialogBackdrop>
    </AlertDialog>
  );
}

function NamespaceGrants({ userId, roles }: { userId: number; roles: Role[] }) {
  const qc = useQueryClient();
  const [roleName, setRoleName] = useState(roles[0]?.name ?? "");
  const [namespace, setNamespace] = useState("");

  const { data: bindings = [] } = useQuery({
    queryKey: ["user-bindings", userId],
    queryFn: () => UsersAPI.bindings(userId),
  });
  const scoped = bindings.filter((b) => b.namespace !== "*");
  const refresh = () => void qc.invalidateQueries({ queryKey: ["user-bindings", userId] });

  const add = useMutation({
    mutationFn: (b: RoleBinding) => UsersAPI.addBinding(userId, b),
    onSuccess: () => {
      setNamespace("");
      refresh();
    },
  });
  const remove = useMutation({
    mutationFn: (b: RoleBinding) => UsersAPI.removeBinding(userId, b.roleName, b.namespace),
    onSuccess: refresh,
  });

  return (
    <div className="space-y-2 pt-3">
      <Separator className="mb-3" />
      <div className="text-[11px] font-semibold uppercase tracking-wider text-foreground/60">
        Namespace grants
      </div>
      {scoped.length === 0 && (
        <div className="text-xs text-foreground/60">No per-namespace grants.</div>
      )}
      <ul className="space-y-1">
        {scoped.map((b) => (
          <li key={`${b.roleName}/${b.namespace}`} className="flex items-center gap-2 text-xs">
            <span className="font-mono">{b.roleName}</span>
            <span className="text-foreground/60">in</span>
            <span className="font-mono">{b.namespace}</span>
            <button
              className="ml-auto rounded p-1 text-foreground/60 hover:text-danger"
              aria-label={`Remove ${b.roleName} in ${b.namespace}`}
              onClick={() => remove.mutate(b)}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </button>
          </li>
        ))}
      </ul>
      <div className="flex items-end gap-2">
        <div className="w-32">
          <select
            value={roleName}
            onChange={(e) => setRoleName(e.target.value)}
            className="w-full rounded border border-border bg-surface px-3 py-2 text-sm text-foreground"
          >
            {roles.map((r) => (
              <option key={r.name} value={r.name}>
                {r.name}
              </option>
            ))}
          </select>
        </div>
        <Input
          className="flex-1"
          value={namespace}
          onChange={(e) => setNamespace(e.target.value)}
          placeholder="namespace"
          aria-label="Grant namespace"
        />
        <Button
          variant="ghost"
          isDisabled={!roleName || !namespace || add.isPending}
          onPress={() => add.mutate({ roleName, namespace })}
        >
          Add
        </Button>
      </div>
      <ErrorLine error={add.error || remove.error} />
    </div>
  );
}

function IdpTab() {
  return (
    <Card className="p-6 text-sm text-foreground/60">
      OIDC identity providers configured in Helm values appear here. UI
      configuration is tracked for v1.1.
    </Card>
  );
}

function ServiceAccountsTab() {
  return (
    <Card className="p-6 text-sm text-foreground/60">
      Service accounts (machine-to-machine API tokens) are tracked for v1.1.
    </Card>
  );
}
