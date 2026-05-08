import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { User } from "@/types";

export type Role = User["role"];

export function useMe() {
  return useQuery({
    queryKey: ["me"],
    queryFn: () => api<User>("/users/me"),
    retry: false,
  });
}

export function hasRole(me: User | undefined, allowed: readonly Role[]): boolean {
  return !!me && allowed.includes(me.role);
}
