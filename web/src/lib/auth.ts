import { useQuery } from "@tanstack/react-query";
import { api, APIError } from "@/lib/api";
import type { User } from "@/types";

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

/**
 * can reports whether the current user holds a permission. It mirrors the
 * server's rbac.Can: for a namespaced check, a cluster-wide ("*" namespace)
 * or exact namespace grant must belong to the explicit target cluster or
 * the "*" wildcard cluster when permissionsByCluster is present. The API
 * enforces authorization independently of these UI checks.
 *
 * When permissionsByCluster is absent (older API), falls back to the flat
 * permissions structure (cluster-agnostic).
 *
 * Pass the explicit target cluster for namespaced checks; omit both ns and
 * cluster for control-plane permissions. Server resource controls consume
 * verified target access through resourceCan/canControl; their useMe calls
 * track identity readiness rather than replacing that permission source.
 */
export function can(
  me: User | undefined,
  perm: string,
  ns?: string,
  cluster?: string,
): boolean {
  // Backward compatibility: when permissionsByCluster is absent, use flat structure.
  if (!me?.permissionsByCluster) {
    const perms = me?.permissions;
    if (!perms) return false;
    const has = (set: string[] | undefined) =>
      !!set && (set.includes("*") || set.includes(perm));
    if (has(perms["*"])) return true;
    if (ns && ns !== "*" && has(perms[ns])) return true;
    return false;
  }

  const perms = me.permissionsByCluster;

  // Helper: does user hold perm cluster-wide ("*" namespace) on cluster ck?
  const cwHolds = (ck: string): boolean => {
    const clusterPerms = perms[ck];
    if (!clusterPerms) return false;
    const nsPerms = clusterPerms["*"];
    if (!nsPerms) return false;
    return nsPerms.includes("*") || nsPerms.includes(perm);
  };

  // A namespace or a cluster argument marks the check as namespaced (the
  // backend's namespaced flag); control-plane checks pass neither.
  const namespaced = ns !== undefined || cluster !== undefined;
  if (!namespaced) {
    // Control-plane perm (no namespace): any cluster's cluster-wide binding
    // grants it.
    for (const ck of Object.keys(perms)) {
      if (cwHolds(ck)) {
        return true;
      }
    }
    return false;
  }

  // Namespaced perm: gated by target cluster (or "*" wildcard cluster).
  // Iterate through target cluster first, then wildcard, to match backend order.
  const targetClusters = cluster !== undefined ? [cluster, "*"] : ["*"];
  for (const ck of targetClusters) {
    if (cwHolds(ck)) {
      return true;
    }
    const clusterPerms = perms[ck];
    if (!clusterPerms) continue;
    const nsPerms = ns && ns !== "*" ? clusterPerms[ns] : undefined;
    if (!nsPerms) continue;
    if (nsPerms.includes("*") || nsPerms.includes(perm)) {
      return true;
    }
  }
  return false;
}
