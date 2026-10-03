import { describe, expect, it } from "vitest";
import { renderHook } from "@testing-library/react";
import type { ReactNode } from "react";
import { QueryClient } from "@tanstack/react-query";
import { ResourceTargetProvider, resourceKey, serverLink, useResourceTarget, useResourceClient, useResourceAccess, useResourcePermissions, resourceCan } from "./resourceTarget";
import { setCurrentCluster } from "./cluster";

const target = { cluster: "remote", namespace: "games", name: "same", uid: "uid-1" };
const access = { canWrite: false, canControl: true, canConsole: true, canDelete: false, isOwner: false, isCollaborator: true, permissions: ["servers:read"] };
function wrapper({ children }: { children: ReactNode }) { return <ResourceTargetProvider target={target} access={access}>{children}</ResourceTargetProvider>; }

describe("resource identity", () => {
  it("constructs a fully scoped deep link without a global selector", () => {
    expect(serverLink(target)).toEqual({ to: "/servers/$name", params: { name: "same" }, search: { cluster: "remote", ns: "games" } });
  });
  it("separates instance data while sharing namespace collections and the live server query", () => {
    const replaced = { ...target, uid: "uid-2" };
    expect(resourceKey(target, "files")).not.toEqual(resourceKey(replaced, "files"));
    expect(resourceKey(target, "server", "same", "games")).toEqual(resourceKey(replaced, "server"));
    expect(resourceKey(target, "backups")).toEqual(resourceKey({ ...target, name: "other" }, "backups", "games"));
    expect(resourceKey(target, "servers")).not.toEqual(resourceKey({ ...target, namespace: "other" }, "servers"));
    expect(resourceKey(target, "server")).not.toEqual(resourceKey({ ...target, cluster: "local" }, "server"));
    const cache = new QueryClient();
    cache.setQueryData(resourceKey(target, "server"), { version: "before" });
    cache.setQueryData(resourceKey(target, "server", "same", "games"), { version: "after" });
    expect(cache.getQueryData(resourceKey(target, "server"))).toEqual({ version: "after" });
    cache.clear();
  });
  it("projects only the exact target capabilities and namespace permissions", () => {
    const { result } = renderHook(() => ({ target: useResourceTarget({ name: "same" }), access: useResourceAccess(), permissions: useResourcePermissions(), client: useResourceClient() }), { wrapper });
    expect(result.current.target).toEqual(target);
    expect(result.current.access).toEqual(access);
    expect(result.current.permissions).toEqual(["servers:read"]);
    expect(resourceCan(result.current.permissions, "servers:write")).toBe(false);
    expect(result.current.client.Logs.podStreamPath("same")).toContain("cluster=remote");
  });
  it("lets a fleet row pass its target and capability explicitly", () => {
    const other = { cluster: "other", namespace: "other-games", name: "different", uid: "uid-other" };
    const { result } = renderHook(() => ({ target: useResourceTarget({ name: "same" }, other), access: useResourceAccess({ ...access, canWrite: true }), permissions: useResourcePermissions(["backups:restore"]), client: useResourceClient(other) }), { wrapper });
    expect(result.current.target).toEqual(other);
    expect(result.current.access?.canWrite).toBe(true);
    expect(result.current.permissions).toEqual(["backups:restore"]);
    expect(result.current.client.Files.downloadURL("different", "x")).toContain("namespace=other-games");
  });
  it("uses deterministic local legacy scope and fails closed on absent authority", () => {
    setCurrentCluster("remembered-remote");
    const { result } = renderHook(() => ({ target: useResourceTarget({ name: "same" }), access: useResourceAccess(), permissions: useResourcePermissions(), client: useResourceClient() }));
    expect(result.current.target.cluster).toBe("local");
    expect(result.current.access).toBeUndefined();
    expect(result.current.permissions).toEqual([]);
    expect(result.current.client.Files.downloadURL("same", "x")).not.toContain("cluster=");
    expect(resourceCan([], "servers:write")).toBe(false);
    expect(resourceCan(["*"], "servers:write")).toBe(true);
    expect(resourceCan(["servers:write"], "servers:write")).toBe(true);
    setCurrentCluster("local");
  });
  it("does not lend one server's UID to another resource in its namespace", () => {
    const { result } = renderHook(() => useResourceTarget({ name: "backup" }), { wrapper });
    expect(result.current).toEqual({ cluster: "remote", namespace: "games", name: "backup", uid: undefined });
  });
});
