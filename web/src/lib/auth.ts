import { useQuery } from "@tanstack/react-query";
import { api, APIError } from "@/lib/api";
import type { User } from "@/types";

export type Role = User["role"];

// Total attempts for /users/me before the route guards give up and show a
// retry card. Three covers a rolling API restart without stalling the UI.
const ME_ATTEMPTS = 3;

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: () => api<User>("/users/me"),
    // A 401 is a definitive answer: retrying only delays the /login redirect.
    // Anything else (API rollout, proxy hiccup, a stalled DB connection) is
    // transient, and must not be mistaken for "you are not allowed here".
    retry: (failureCount, err) =>
      !(err instanceof APIError && err.status === 401) && failureCount < ME_ATTEMPTS,
  });
}

export function hasRole(me: User | undefined, allowed: readonly Role[]): boolean {
  return !!me && allowed.includes(me.role);
}

/**
 * can reports whether the current user holds a permission. It mirrors the
 * server's rbac.Can: a cluster-wide ("*") grant of the permission (or the
 * "*" wildcard) suffices anywhere; for a namespaced action, a grant in the
 * target namespace also counts. The API is always the real enforcer — this
 * only drives what the UI shows.
 */
export function can(me: User | undefined, perm: string, ns?: string): boolean {
  const perms = me?.permissions;
  if (!perms) return false;
  const has = (set: string[] | undefined) =>
    !!set && (set.includes("*") || set.includes(perm));
  if (has(perms["*"])) return true;
  if (ns && ns !== "*" && has(perms[ns])) return true;
  return false;
}
