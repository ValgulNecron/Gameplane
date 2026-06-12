import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import * as Dialog from "@radix-ui/react-dialog";
import {
  FileText,
  KeyRound,
  MoreHorizontal,
  Pencil,
  Plus,
  Search,
  Trash2,
} from "lucide-react";
import { Link } from "@tanstack/react-router";
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
import { useMe } from "@/lib/auth";
import { Users as UsersAPI, type UserCreate, type UserUpdate } from "@/lib/endpoints";
import { cn, formatRelative } from "@/lib/utils";
import type { ExtendedUser, UserRole } from "@/types";

type Tab = "users" | "roles" | "service" | "idp";

const ROLES: UserRole[] = ["viewer", "operator", "admin"];
const MIN_PASSWORD_LEN = 12; // mirrors api/internal/handlers/users.go

const roleColor: Record<UserRole, string> = {
  admin: "bg-primary/15 text-primary",
  operator: "bg-violet/15 text-violet",
  viewer: "bg-muted/20 text-muted",
};

export function UsersPage() {
  const qc = useQueryClient();
  const { data: me } = useMe();
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

  const counts = useMemo(
    () => ({
      users: users.length,
      roles: ROLES.length,
      service: 0,
      idp: 0,
    }),
    [users],
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
        subtitle="Manage access to the Kestrel control plane."
        actions={
          <div className="flex items-center gap-2">
            <Button variant="outline" asChild>
              <Link to="/admin/audit">
                <FileText className="h-4 w-4" /> Audit log
              </Link>
            </Button>
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
            { key: "service", label: "Service accounts",   count: counts.service },
            { key: "idp",     label: "Identity providers", count: counts.idp },
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
      <td className="px-4 py-3 text-muted">{u.provider ?? "local"}</td>
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
  return <div className="pt-2 text-xs text-destructive">{text}</div>;
}

function InviteModal({
  onClose,
  onCreated,
}: {
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
              <div className="pt-1 text-[11px] text-destructive">
                At least {MIN_PASSWORD_LEN} characters.
              </div>
            )}
          </div>
          <FieldLabel label="Role">
            <Select
              value={form.role}
              onValueChange={(v) => setForm({ ...form, role: v as UserRole })}
              options={ROLES.map((r) => ({ value: r, label: r }))}
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
  isMe,
  onClose,
  onSaved,
}: {
  user: ExtendedUser;
  isMe: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [displayName, setDisplayName] = useState(user.displayName ?? "");
  const [email, setEmail] = useState(user.email ?? "");
  const [role, setRole] = useState<UserRole>(user.role);

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

  // Demoting yourself out of admin would lock you out; the API rejects
  // it too — disable client-side so the path is obvious.
  const wouldDemoteSelf = isMe && user.role === "admin" && role !== "admin";
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
          <FieldLabel label="Role">
            <Select
              value={role}
              onValueChange={(v) => setRole(v as UserRole)}
              options={ROLES.map((r) => ({ value: r, label: r }))}
            />
          </FieldLabel>
          {wouldDemoteSelf && (
            <div className="pt-1 text-[11px] text-destructive">
              You can’t demote yourself out of admin.
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
            <div className="pt-1 text-[11px] text-destructive">
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
    <span className="text-destructive">You can’t delete your own account.</span>
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
  const roles = [
    { name: "admin", desc: "Full access to all resources, including users and global config." },
    { name: "operator", desc: "Manage game servers, backups, and templates." },
    { name: "viewer", desc: "Read-only access across the control panel." },
  ];
  return (
    <div className="grid gap-4 md:grid-cols-3">
      {roles.map((r) => (
        <Card key={r.name} className="space-y-2 p-4">
          <div className="flex items-center justify-between">
            <span className="font-mono text-sm">{r.name}</span>
          </div>
          <p className="text-xs text-muted">{r.desc}</p>
        </Card>
      ))}
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
