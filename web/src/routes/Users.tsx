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
  Tab,
  Table,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownSection,
  DropdownItem,
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
  Alert,
  Label,
  Description,
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
import { PageHeader } from "@/components/PageHeader";
import { RoleEditorModal } from "@/components/hero/RoleEditorModal";
import { APIError } from "@/lib/api";
import { useMe, can } from "@/lib/auth";
import {
  Users as UsersAPI,
  Roles as RolesAPI,
  type UserCreate,
  type UserUpdate,
} from "@/lib/endpoints";
import { formatRelative } from "@/lib/utils";
import type { ExtendedUser, Role, RoleBinding } from "@/types";

type Tab = "users" | "roles" | "service" | "idp";

const MIN_PASSWORD_LEN = 12;

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
  const [tab, setTab] = useState<Tab>("users");
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

  return (
    <div className="space-y-6 p-6">
      <PageHeader
        title="Users & RBAC"
        subtitle="Manage access to the Gameplane control plane."
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
          onSelectionChange={(key) => setTab(key as Tab)}
          variant="secondary"
        >
          <Tabs.List>
            <Tab id="users">
              <div className="flex items-center gap-2">
                <span>Users</span>
                <span className="text-xs text-foreground/60">({counts.users})</span>
              </div>
            </Tab>
            <Tab id="roles">
              <div className="flex items-center gap-2">
                <span>Roles</span>
                <span className="text-xs text-foreground/60">({counts.roles})</span>
              </div>
            </Tab>
            <Tab id="service">Service accounts</Tab>
            <Tab id="idp">Identity providers</Tab>
          </Tabs.List>
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

      {error instanceof APIError && (
        <Alert status="danger">
          <Alert.Indicator />
          <Alert.Title>Error loading users</Alert.Title>
          {error.body && <Alert.Description>{error.body}</Alert.Description>}
        </Alert>
      )}

      {tab === "users" && (
        <Table.Root>
          <Table.ScrollContainer>
            <Table.Content aria-label="Users list">
              <Table.Header>
                <Table.Column id="user">User</Table.Column>
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
                  <span className="rounded px-2 py-0.5 text-[10px] font-mono uppercase bg-primary/10 text-primary">
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
                    <DropdownTrigger>
                      <Button
                        isIconOnly
                        variant="ghost"
                        size="sm"
                        aria-label={`Actions for ${u.username}`}
                      >
                        <MoreHorizontal className="h-4 w-4" />
                      </Button>
                    </DropdownTrigger>
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
                      >
                        <div className="flex items-center gap-2">
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
                  </Dropdown>
                </Table.Cell>
              </Table.Row>
                ))}
              </Table.Body>
            </Table.Content>
          </Table.ScrollContainer>
        </Table.Root>
      )}

      {tab === "roles" && <RolesTab />}
      {tab === "service" && <ServiceAccountsTab />}
      {tab === "idp" && <IdpTab />}

      {inviting && (
        <InviteUserForm
          roles={roles}
          onClose={() => setInviting(false)}
          onCreated={() => {
            invalidate();
            setInviting(false);
          }}
        />
      )}
      {editing && (
        <EditUserForm
          user={editing}
          roles={roles}
          isMe={!!me && me.id === editing.id}
          onClose={() => setEditing(null)}
          onSaved={() => {
            invalidate();
            setEditing(null);
          }}
        />
      )}
      {resetting && (
        <ResetPasswordForm
          user={resetting}
          onClose={() => setResetting(null)}
          onDone={() => setResetting(null)}
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

function InviteUserForm({
  roles,
  onClose,
  onCreated,
}: {
  roles: Role[];
  onClose: () => void;
  onCreated: () => void;
}) {
  const [form, setForm] = useState<UserCreate>({
    username: "",
    displayName: "",
    email: "",
    password: "",
    role: "viewer",
  });
  const create = useMutation({
    mutationFn: () =>
      UsersAPI.create({
        username: form.username,
        displayName: form.displayName || undefined,
        email: form.email || undefined,
        password: form.password || undefined,
        role: form.role,
      }),
    onSuccess: onCreated,
  });

  const passwordTooShort =
    !!form.password && form.password.length < MIN_PASSWORD_LEN;
  const submitDisabled =
    !form.username || passwordTooShort || create.isPending;

  return (
    <Modal isOpen onOpenChange={(open) => !open && onClose()}>
      <ModalBackdrop isDismissable={!create.isPending} isKeyboardDismissDisabled={create.isPending} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Invite user</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <Description>Create a local account. Leave password blank to send an OIDC invite later.</Description>

            <div>
              <Label htmlFor="invite-username" className="text-xs">
                Username
              </Label>
              <Input
                id="invite-username"
                autoFocus
                value={form.username}
                onChange={(e) => setForm({ ...form, username: e.target.value })}
                placeholder="alice"
                className="mt-1"
                disabled={create.isPending}
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div>
                <Label htmlFor="invite-display-name" className="text-xs">
                  Display name
                </Label>
                <Input
                  id="invite-display-name"
                  value={form.displayName ?? ""}
                  onChange={(e) => setForm({ ...form, displayName: e.target.value })}
                  placeholder="Alice Operator"
                  className="mt-1"
                  disabled={create.isPending}
                />
              </div>

              <div>
                <Label htmlFor="invite-email" className="text-xs">
                  Email
                </Label>
                <Input
                  id="invite-email"
                  type="email"
                  value={form.email ?? ""}
                  onChange={(e) => setForm({ ...form, email: e.target.value })}
                  placeholder="alice@example.com"
                  className="mt-1"
                  disabled={create.isPending}
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
                  type="password"
                  value={form.password ?? ""}
                  onChange={(e) => setForm({ ...form, password: e.target.value })}
                  placeholder={`At least ${MIN_PASSWORD_LEN} characters`}
                  className="mt-1"
                  disabled={create.isPending}
                />
                {passwordTooShort && (
                  <p className="pt-1 text-[11px] text-danger">
                    At least {MIN_PASSWORD_LEN} characters.
                  </p>
                )}
              </div>

              <div>
                <Label htmlFor="invite-role" className="text-xs">
                  Role
                </Label>
                <select
                  id="invite-role"
                  className="mt-1 w-full rounded border border-border bg-surface px-3 py-2 text-sm text-foreground"
                  value={form.role}
                  onChange={(e) => setForm({ ...form, role: e.target.value })}
                  disabled={create.isPending}
                >
                  {roles.map((r) => (
                    <option key={r.name} value={r.name}>
                      {r.name}
                    </option>
                  ))}
                </select>
              </div>
            </div>

            <ErrorLine error={create.error} />
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              onPress={onClose}
              isDisabled={create.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              isDisabled={submitDisabled}
              onPress={() => create.mutate()}
            >
              {create.isPending ? "Creating…" : "Create user"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}

function EditUserForm({
  user,
  roles,
  isMe,
  onClose,
  onSaved,
}: {
  user: ExtendedUser;
  roles: Role[];
  isMe: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [displayName, setDisplayName] = useState(user.displayName ?? "");
  const [email, setEmail] = useState(user.email ?? "");
  const [role, setRole] = useState<string>(user.role);
  const [resetFor, setResetFor] = useState(user);
  if (user !== resetFor) {
    setResetFor(user);
    setDisplayName(user.displayName ?? "");
    setEmail(user.email ?? "");
    setRole(user.role);
  }

  const dirty: UserUpdate = useMemo(() => {
    const out: UserUpdate = {};
    if (displayName !== (user.displayName ?? "")) out.displayName = displayName;
    if (email !== (user.email ?? "")) out.email = email;
    if (role !== user.role) out.role = role;
    return out;
  }, [displayName, email, role, user]);

  const save = useMutation({
    mutationFn: () => UsersAPI.update(user.id, dirty),
    onSuccess: onSaved,
  });

  const wouldDemoteSelf =
    isMe && role !== user.role && !roleGrantsUserManagement(roles, role);
  const noChanges = Object.keys(dirty).length === 0;

  return (
    <Modal isOpen onOpenChange={(open) => !open && onClose()}>
      <ModalBackdrop isDismissable={!save.isPending} isKeyboardDismissDisabled={save.isPending} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Edit user</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <Description>{user.username}</Description>

            <div>
              <Label htmlFor="edit-display-name" className="text-xs">
                Display name
              </Label>
              <Input
                id="edit-display-name"
                autoFocus
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                className="mt-1"
                disabled={save.isPending}
              />
            </div>

            <div>
              <Label htmlFor="edit-email" className="text-xs">
                Email
              </Label>
              <Input
                id="edit-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                className="mt-1"
                disabled={save.isPending}
              />
            </div>

            <div>
              <Label htmlFor="edit-role" className="text-xs">
                Primary role (cluster-wide)
              </Label>
              <select
                id="edit-role"
                className="mt-1 w-full rounded border border-border bg-surface px-3 py-2 text-sm text-foreground"
                value={role}
                onChange={(e) => setRole(e.target.value)}
                disabled={save.isPending}
              >
                {roles.map((r) => (
                  <option key={r.name} value={r.name}>
                    {r.name}
                  </option>
                ))}
              </select>
              {wouldDemoteSelf && (
                <p className="pt-1 text-[11px] text-danger">
                  You can't remove your own ability to manage users.
                </p>
              )}
            </div>

            <div className="border-t border-divider my-2" />
            <NamespaceGrants userId={user.id} roles={roles} />

            <ErrorLine error={save.error} />
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              onPress={onClose}
              isDisabled={save.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              isDisabled={save.isPending || noChanges || wouldDemoteSelf}
              onPress={() => save.mutate()}
            >
              {save.isPending ? "Saving…" : "Save changes"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
  );
}

function ResetPasswordForm({
  user,
  onClose,
  onDone,
}: {
  user: ExtendedUser;
  onClose: () => void;
  onDone: () => void;
}) {
  const [pw, setPw] = useState("");
  const reset = useMutation({
    mutationFn: () => UsersAPI.resetPassword(user.id, pw),
    onSuccess: onDone,
  });
  const tooShort = pw.length > 0 && pw.length < MIN_PASSWORD_LEN;
  const disabled = pw.length < MIN_PASSWORD_LEN || reset.isPending;

  return (
    <Modal isOpen onOpenChange={(open) => !open && onClose()}>
      <ModalBackdrop isDismissable={!reset.isPending} isKeyboardDismissDisabled={reset.isPending} />
      <ModalContainer>
        <ModalDialog>
          <ModalHeader>
            <ModalHeading>Reset password for {user.username}</ModalHeading>
          </ModalHeader>

          <ModalBody className="gap-4">
            <Description>They will need to sign in again with the new password.</Description>

            <div>
              <Label htmlFor="reset-password" className="text-xs">
                New password
              </Label>
              <Input
                id="reset-password"
                type="password"
                autoFocus
                value={pw}
                onChange={(e) => setPw(e.target.value)}
                placeholder={`At least ${MIN_PASSWORD_LEN} characters`}
                className="mt-1"
                disabled={reset.isPending}
              />
              {tooShort && (
                <p className="pt-1 text-[11px] text-danger">
                  At least {MIN_PASSWORD_LEN} characters.
                </p>
              )}
            </div>

            <ErrorLine error={reset.error} />
          </ModalBody>

          <ModalFooter className="flex items-center justify-end gap-2">
            <Button
              variant="secondary"
              size="sm"
              onPress={onClose}
              isDisabled={reset.isPending}
            >
              Cancel
            </Button>
            <Button
              size="sm"
              variant="primary"
              isDisabled={disabled}
              onPress={() => reset.mutate()}
            >
              {reset.isPending ? "Resetting…" : "Reset password"}
            </Button>
          </ModalFooter>
        </ModalDialog>
      </ModalContainer>
    </Modal>
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
      <AlertDialogBackdrop isDismissable={!remove.isPending} isKeyboardDismissDisabled={remove.isPending} />
      <AlertDialogContainer>
        <Modal>
          <ModalDialog role="alertdialog" className="max-w-md">
            <ModalHeader>
              <ModalHeading>Delete {user.username}?</ModalHeading>
            </ModalHeader>
            <ModalBody>
              {isMe ? (
                <p className="text-sm text-danger">You can't delete your own account.</p>
              ) : (
                <div className="space-y-2">
                  <p className="text-sm">
                    Their sessions will be revoked and they'll lose access immediately. This cannot be undone.
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
      <AlertDialogBackdrop isDismissable={!remove.isPending} isKeyboardDismissDisabled={remove.isPending} />
      <AlertDialogContainer>
        <Modal>
          <ModalDialog role="alertdialog" className="max-w-md">
            <ModalHeader>
              <ModalHeading>Delete role {role.name}?</ModalHeading>
            </ModalHeader>
            <ModalBody>
              <div className="space-y-2">
                <p className="text-sm">
                  This can't be undone. Roles assigned to a user can't be deleted.
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
    <div className="space-y-2 border-t border-border pt-3">
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
