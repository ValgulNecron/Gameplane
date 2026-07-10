import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { Link } from "@tanstack/react-router";
import * as Dialog from "@radix-ui/react-dialog";
import {
  KeyRound,
  MoreHorizontal,
  Pencil,
  Plus,
  ScrollText,
  Search,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { Card } from "@/components/ui/card";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { FieldLabel } from "@/components/ui/field";
import { TabBar } from "@/components/ui/tabs";
import { PageHeader } from "@/components/PageHeader";
import { APIError } from "@/lib/api";
import { useMe, can } from "@/lib/auth";
import {
  Users as UsersAPI,
  Roles as RolesAPI,
  type UserCreate,
  type UserUpdate,
} from "@/lib/endpoints";
import { cn, formatRelative } from "@/lib/utils";
import type { ExtendedUser, PermissionGroup, Role, RoleBinding } from "@/types";

type Tab = "users" | "roles" | "service" | "idp";

const MIN_PASSWORD_LEN = 12; // mirrors api/internal/handlers/users.go

const roleColor: Record<string, string> = {
  admin: "bg-primary/15 text-primary",
  operator: "bg-violet/15 text-violet",
  viewer: "bg-muted/20 text-muted",
};

function useRolesQuery() {
  return useQuery({ queryKey: ["roles"], queryFn: () => RolesAPI.list() });
}

// roleGrantsUserManagement mirrors the server guard: a role can manage
// users if it holds users:manage or the "*" wildcard.
function roleGrantsUserManagement(roles: Role[], name: string): boolean {
  const r = roles.find((x) => x.name === name);
  return !!r && (r.permissions.includes("*") || r.permissions.includes("users:manage"));
}

export function UsersPage() {
  const qc = useQueryClient();
  const { data: me } = useMe();
  // Surface a quick jump to the audit log (design parity), but only for
  // users who can actually read it — the /admin/audit route is gated on
  // audit:read, so a link for anyone else would dead-end.
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

  // Fire-and-forget; the next render reflects whatever resolves.
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
                className="inline-flex h-9 items-center gap-1.5 rounded-md border border-border px-3 text-sm font-medium text-fg transition-colors hover:bg-surface"
              >
                <ScrollText className="h-4 w-4" /> Audit log
              </Link>
            )}
            <Button onClick={() => setInviting(true)}>
              <Plus className="h-4 w-4" /> Invite user
            </Button>
          </div>
        }
      />

      <div className="flex flex-wrap items-center gap-3">
        <TabBar
          items={[
            { key: "users",   label: "Users",              count: counts.users },
            { key: "roles",   label: "Roles",              count: counts.roles },
            { key: "service", label: "Service accounts" },
            { key: "idp",     label: "Identity providers" },
          ]}
          value={tab}
          onChange={setTab}
        />
        <div className="relative ml-auto w-64">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" />
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
        <Card className="p-3 text-sm text-warning">{error.body || "Failed to load users."}</Card>
      )}

      {tab === "users" && (
        <Card>
          <table className="w-full text-sm">
            <thead className="bg-surface/70 text-left text-[11px] uppercase tracking-wider text-muted">
              <tr>
                <th className="px-4 py-3">User</th>
                <th className="px-4 py-3">Role</th>
                <th className="px-4 py-3">Provider</th>
                <th className="px-4 py-3">Created</th>
                <th className="px-4 py-3" />
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {visible.length === 0 && (
                <tr>
                  <td colSpan={5} className="px-4 py-10 text-center text-muted">
                    No entries.
                  </td>
                </tr>
              )}
              {visible.map((u) => (
                <UserRow
                  key={u.id}
                  u={u}
                  isMe={!!me && me.id === u.id}
                  onEdit={() => setEditing(u)}
                  onResetPassword={() => setResetting(u)}
                  onDelete={() => setDeleting(u)}
                />
              ))}
            </tbody>
          </table>
        </Card>
      )}

      {tab === "roles" && <RolesTab />}
      {tab === "service" && <ServiceAccountsTab />}
      {tab === "idp" && <IdpTab />}

      {inviting && (
        <InviteModal
          roles={roles}
          onClose={() => setInviting(false)}
          onCreated={() => {
            invalidate();
            setInviting(false);
          }}
        />
      )}
      {editing && (
        <EditUserModal
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
        <ResetPasswordModal
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

function UserRow({
  u,
  isMe,
  onEdit,
  onResetPassword,
  onDelete,
}: {
  u: ExtendedUser;
  isMe: boolean;
  onEdit: () => void;
  onResetPassword: () => void;
  onDelete: () => void;
}) {
  const initials = (u.displayName || u.username).slice(0, 2).toUpperCase();
  return (
    <tr className="hover:bg-surface/40">
      <td className="px-4 py-3">
        <div className="flex items-center gap-3">
          <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/20 font-mono text-xs text-primary">
            {initials}
          </div>
          <div className="min-w-0">
            <div className="truncate font-mono text-sm text-fg">
              {u.username}
              {isMe && (
                <span className="ml-2 rounded bg-muted/20 px-1.5 py-0.5 text-[10px] text-muted">
                  you
                </span>
              )}
            </div>
            <div className="truncate text-[11px] text-muted">
              {u.displayName || u.email || "—"}
            </div>
            {/* Email subline — skipped when the email is already what the
                line above shows (no display name set). */}
            {u.email && u.displayName && u.displayName !== u.email && (
              <div className="truncate text-[11px] text-muted">{u.email}</div>
            )}
          </div>
        </div>
      </td>
      <td className="px-4 py-3">
        <span
          className={cn(
            "rounded px-2 py-0.5 text-[10px] font-mono uppercase",
            roleColor[u.role] ?? "bg-muted/20 text-muted",
          )}
        >
          {u.role}
        </span>
      </td>
      <td className="px-4 py-3">
        <span className="rounded px-2 py-0.5 text-[10px] uppercase text-muted ring-1 ring-border">
          {u.provider === "oidc" ? "OIDC" : u.provider === "pending" ? "Pending" : "Local"}
        </span>
      </td>
      <td className="px-4 py-3 text-muted">{formatRelative(u.createdAt)}</td>
      <td className="px-4 py-3 text-right">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              className="rounded p-1 text-muted hover:bg-border hover:text-fg focus:outline-none focus:ring-1 focus:ring-primary"
              aria-label={`Actions for ${u.username}`}
            >
              <MoreHorizontal className="h-4 w-4" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent>
            <DropdownMenuItem
              icon={<Pencil className="h-4 w-4" />}
              onSelect={onEdit}
              label="Edit user"
            />
            <DropdownMenuItem
              icon={<KeyRound className="h-4 w-4" />}
              onSelect={onResetPassword}
              label="Reset password"
              disabled={u.provider === "oidc"}
              hint={u.provider === "oidc" ? "Account is OIDC-managed" : undefined}
            />
            <DropdownMenuSeparator />
            <DropdownMenuItem
              icon={<Trash2 className="h-4 w-4" />}
              onSelect={onDelete}
              label="Delete user"
              destructive
              disabled={isMe}
              hint={isMe ? "You can’t delete your own account" : undefined}
            />
          </DropdownMenuContent>
        </DropdownMenu>
      </td>
    </tr>
  );
}

function ModalShell({
  open,
  onOpenChange,
  title,
  description,
  children,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
  title: string;
  description?: ReactNode;
  children: ReactNode;
}) {
  return (
    <Dialog.Root open={open} onOpenChange={onOpenChange}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 z-40 bg-black/60" />
        <Dialog.Content className="fixed left-1/2 top-1/2 z-50 w-[480px] max-w-[calc(100vw-2rem)] -translate-x-1/2 -translate-y-1/2 rounded-lg border border-border bg-card p-5 text-fg shadow-2xl">
          <Dialog.Title className="text-base font-semibold">{title}</Dialog.Title>
          {description && (
            <Dialog.Description asChild>
              <div className="pt-1 text-sm text-muted">{description}</div>
            </Dialog.Description>
          )}
          <div className="pt-4">{children}</div>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

function ErrorLine({ error }: { error: unknown }) {
  if (!error) return null;
  const text =
    error instanceof APIError
      ? error.body || `Request failed (${error.status})`
      : (error as Error).message;
  return <div className="pt-2 text-xs text-danger">{text}</div>;
}

function InviteModal({
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
    <ModalShell
      open
      onOpenChange={(v) => !v && onClose()}
      title="Invite user"
      description="Create a local account. Leave password blank to send an OIDC invite later."
    >
      <div className="space-y-3">
        <FieldLabel label="Username">
          <Input
            autoFocus
            value={form.username}
            onChange={(e) => setForm({ ...form, username: e.target.value })}
            placeholder="alice"
          />
        </FieldLabel>
        <div className="grid grid-cols-2 gap-3">
          <FieldLabel label="Display name">
            <Input
              value={form.displayName ?? ""}
              onChange={(e) => setForm({ ...form, displayName: e.target.value })}
              placeholder="Alice Operator"
            />
          </FieldLabel>
          <FieldLabel label="Email">
            <Input
              type="email"
              value={form.email ?? ""}
              onChange={(e) => setForm({ ...form, email: e.target.value })}
              placeholder="alice@example.com"
            />
          </FieldLabel>
        </div>
        <div className="grid grid-cols-2 gap-3">
          <div>
            <FieldLabel label="Initial password">
              <Input
                type="password"
                value={form.password ?? ""}
                onChange={(e) => setForm({ ...form, password: e.target.value })}
                placeholder={`At least ${MIN_PASSWORD_LEN} characters`}
              />
            </FieldLabel>
            {passwordTooShort && (
              <div className="pt-1 text-[11px] text-danger">
                At least {MIN_PASSWORD_LEN} characters.
              </div>
            )}
          </div>
          <FieldLabel label="Role">
            <Select
              value={form.role}
              onValueChange={(v) => setForm({ ...form, role: v })}
              options={roles.map((r) => ({ value: r.name, label: r.name }))}
            />
          </FieldLabel>
        </div>
        <ErrorLine error={create.error} />
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={onClose} disabled={create.isPending}>
            Cancel
          </Button>
          <Button onClick={() => create.mutate()} disabled={submitDisabled}>
            {create.isPending ? "Creating…" : "Create user"}
          </Button>
        </div>
      </div>
    </ModalShell>
  );
}

function EditUserModal({
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

  useEffect(() => {
    setDisplayName(user.displayName ?? "");
    setEmail(user.email ?? "");
    setRole(user.role);
  }, [user]);

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

  // Switching your own primary role to one that can't manage users would
  // lock you out of RBAC; the API rejects it too — surface it here.
  const wouldDemoteSelf =
    isMe && role !== user.role && !roleGrantsUserManagement(roles, role);
  const noChanges = Object.keys(dirty).length === 0;

  return (
    <ModalShell
      open
      onOpenChange={(v) => !v && onClose()}
      title="Edit user"
      description={user.username}
    >
      <div className="space-y-3">
        <FieldLabel label="Display name">
          <Input
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            autoFocus
          />
        </FieldLabel>
        <FieldLabel label="Email">
          <Input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
          />
        </FieldLabel>
        <div>
          <FieldLabel label="Primary role (cluster-wide)">
            <Select
              value={role}
              onValueChange={(v) => setRole(v)}
              options={roles.map((r) => ({ value: r.name, label: r.name }))}
            />
          </FieldLabel>
          {wouldDemoteSelf && (
            <div className="pt-1 text-[11px] text-danger">
              You can’t remove your own ability to manage users.
            </div>
          )}
        </div>
        <ErrorLine error={save.error} />
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={onClose} disabled={save.isPending}>
            Cancel
          </Button>
          <Button
            onClick={() => save.mutate()}
            disabled={save.isPending || noChanges || wouldDemoteSelf}
          >
            {save.isPending ? "Saving…" : "Save changes"}
          </Button>
        </div>

        <NamespaceGrants userId={user.id} roles={roles} />
      </div>
    </ModalShell>
  );
}

function ResetPasswordModal({
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
    <ModalShell
      open
      onOpenChange={(v) => !v && onClose()}
      title={`Reset password for ${user.username}`}
      description="They will need to sign in again with the new password."
    >
      <div className="space-y-3">
        <div>
          <FieldLabel label="New password">
            <Input
              type="password"
              value={pw}
              onChange={(e) => setPw(e.target.value)}
              autoFocus
              placeholder={`At least ${MIN_PASSWORD_LEN} characters`}
            />
          </FieldLabel>
          {tooShort && (
            <div className="pt-1 text-[11px] text-danger">
              At least {MIN_PASSWORD_LEN} characters.
            </div>
          )}
        </div>
        <ErrorLine error={reset.error} />
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={onClose} disabled={reset.isPending}>
            Cancel
          </Button>
          <Button onClick={() => reset.mutate()} disabled={disabled}>
            {reset.isPending ? "Setting…" : "Set new password"}
          </Button>
        </div>
      </div>
    </ModalShell>
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
  const description = isMe ? (
    <span className="text-danger">You can’t delete your own account.</span>
  ) : (
    <>
      Their sessions will be revoked and they’ll lose access immediately. This
      cannot be undone.
      {remove.error && <ErrorLine error={remove.error} />}
    </>
  );
  return (
    <ConfirmDialog
      open
      onOpenChange={(v) => !v && onClose()}
      title={`Delete ${user.username}?`}
      description={description}
      confirmLabel={isMe ? "Cannot delete" : "Delete user"}
      destructive
      busy={remove.isPending}
      onConfirm={() => {
        if (!isMe) remove.mutate();
      }}
    />
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
          <Button onClick={() => setCreating(true)}>
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
                <span className="rounded bg-muted/20 px-1.5 py-0.5 text-[10px] uppercase text-muted">
                  built-in
                </span>
              )}
            </div>
            <p className="min-h-8 text-xs text-muted">{r.description || "—"}</p>
            <div className="text-[11px] text-muted">
              {r.permissions.includes("*")
                ? "all permissions"
                : `${r.permissions.length} permission${r.permissions.length === 1 ? "" : "s"}`}
            </div>
            {canManage && (
              <div className="flex gap-2 pt-1">
                {/* The admin role's wildcard is immutable; everything else
                    is editable. Only custom roles can be deleted. */}
                {r.name !== "admin" && (
                  <Button variant="ghost" className="h-7 px-2 text-xs" onClick={() => setEditing(r)}>
                    <Pencil className="h-3.5 w-3.5" /> Edit
                  </Button>
                )}
                {!r.builtin && (
                  <Button
                    variant="ghost"
                    className="h-7 px-2 text-xs text-danger"
                    onClick={() => setDeleting(r)}
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
          role={editing}
          groups={groups}
          onClose={() => {
            setCreating(false);
            setEditing(null);
          }}
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

function RoleEditorModal({
  role,
  groups,
  onClose,
  onSaved,
}: {
  role: Role | null;
  groups: PermissionGroup[];
  onClose: () => void;
  onSaved: () => void;
}) {
  const creating = role === null;
  const [name, setName] = useState(role?.name ?? "");
  const [description, setDescription] = useState(role?.description ?? "");
  const [selected, setSelected] = useState<Set<string>>(
    new Set(role?.permissions ?? []),
  );

  const toggle = (key: string) =>
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });

  const save = useMutation({
    mutationFn: () => {
      const permissions = [...selected];
      return role
        ? RolesAPI.update(role.name, { description, permissions })
        : RolesAPI.create({ name, description, permissions });
    },
    onSuccess: onSaved,
  });

  const nameValid = /^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$/.test(name);
  const submitDisabled = (creating && !nameValid) || save.isPending;

  return (
    <ModalShell
      open
      onOpenChange={(v) => !v && onClose()}
      title={role ? `Edit role: ${role.name}` : "New role"}
      description="Grant a curated set of permissions."
    >
      <div className="space-y-3">
        {creating && (
          <FieldLabel label="Name">
            <Input
              autoFocus
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="support"
            />
          </FieldLabel>
        )}
        <FieldLabel label="Description">
          <Input
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="What this role is for"
          />
        </FieldLabel>
        <div className="max-h-72 space-y-3 overflow-auto rounded-md border border-border p-3">
          {groups.map((g) => (
            <div key={g.resource}>
              <div className="pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted">
                {g.label}
              </div>
              <div className="grid grid-cols-1 gap-1 sm:grid-cols-2">
                {g.permissions.map((p) => (
                  <label
                    key={p.key}
                    className="flex cursor-pointer items-center gap-2 text-xs text-fg"
                  >
                    <input
                      type="checkbox"
                      className="accent-primary"
                      checked={selected.has(p.key)}
                      onChange={() => toggle(p.key)}
                    />
                    <span className="font-mono">{p.key}</span>
                    {p.namespaced && (
                      <span className="rounded bg-muted/15 px-1 text-[9px] text-muted">ns</span>
                    )}
                  </label>
                ))}
              </div>
            </div>
          ))}
        </div>
        <ErrorLine error={save.error} />
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="ghost" onClick={onClose} disabled={save.isPending}>
            Cancel
          </Button>
          <Button onClick={() => save.mutate()} disabled={submitDisabled}>
            {save.isPending ? "Saving…" : creating ? "Create role" : "Save role"}
          </Button>
        </div>
      </div>
    </ModalShell>
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
    <ConfirmDialog
      open
      onOpenChange={(v) => !v && onClose()}
      title={`Delete role ${role.name}?`}
      description={
        <>
          This can’t be undone. Roles assigned to a user can’t be deleted.
          {remove.error && <ErrorLine error={remove.error} />}
        </>
      }
      confirmLabel="Delete role"
      destructive
      busy={remove.isPending}
      onConfirm={() => remove.mutate()}
    />
  );
}

// NamespaceGrants edits a user's per-namespace role bindings (the
// cluster-wide grant is the primary role, edited above).
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
      <div className="text-[11px] font-semibold uppercase tracking-wider text-muted">
        Namespace grants
      </div>
      {scoped.length === 0 && (
        <div className="text-xs text-muted">No per-namespace grants.</div>
      )}
      <ul className="space-y-1">
        {scoped.map((b) => (
          <li key={`${b.roleName}/${b.namespace}`} className="flex items-center gap-2 text-xs">
            <span className="font-mono">{b.roleName}</span>
            <span className="text-muted">in</span>
            <span className="font-mono">{b.namespace}</span>
            <button
              className="ml-auto rounded p-1 text-muted hover:text-danger"
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
          <Select
            value={roleName}
            onValueChange={setRoleName}
            options={roles.map((r) => ({ value: r.name, label: r.name }))}
          />
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
          disabled={!roleName || !namespace || add.isPending}
          onClick={() => add.mutate({ roleName, namespace })}
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
    <Card className="p-6 text-sm text-muted">
      OIDC identity providers configured in Helm values appear here. UI
      configuration is tracked for v1.1.
    </Card>
  );
}

function ServiceAccountsTab() {
  return (
    <Card className="p-6 text-sm text-muted">
      Service accounts (machine-to-machine API tokens) are tracked for v1.1.
    </Card>
  );
}
