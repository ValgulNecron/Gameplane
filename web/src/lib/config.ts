import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/lib/api";

// Section schemas mirror api/internal/handlers/config.go validators.
// Keep these in sync if you add or rename a field server-side.
export interface GeneralCfg {
  instanceName: string;
  externalURL: string;
  defaultNamespace: string;
}

export type AuthKind = "local" | "oidc" | "google" | "github";

export interface AuthProvider {
  name: string;
  kind: AuthKind;
  enabled: boolean;
  configRef?: string;
}

export interface AuthCfg {
  providers: AuthProvider[];
}

export type SinkKind = "discord" | "slack" | "smtp" | "webhook";

export interface NotifSink {
  name: string;
  kind: SinkKind;
  enabled: boolean;
}

export interface NotificationsCfg {
  sinks: NotifSink[];
}

export interface TelemetryCfg {
  sendMetrics: boolean;
}

export type UpdateChannel = "stable" | "beta" | "nightly";

export interface UpdatesCfg {
  channel: UpdateChannel;
}

// AllConfig is the shape of GET /admin/config — every section is optional
// because the row only exists once it's been written. Defaults are
// supplied by the section components, not by the API.
//
// Backup destinations are NOT in this config — they live as labelled
// Kubernetes Secrets, served via /backup-destinations.
export interface AllConfig {
  general?: GeneralCfg;
  auth?: AuthCfg;
  notifications?: NotificationsCfg;
  telemetry?: TelemetryCfg;
  updates?: UpdatesCfg;
}

const KEY = ["config"] as const;

export function useConfig() {
  return useQuery({
    queryKey: KEY,
    queryFn: () => api<AllConfig>("/admin/config"),
  });
}

// useUpdateConfigSection PUTs a single section's blob and invalidates the
// cached AllConfig so the GET re-fires on success. The mutation type is
// section-specific so callers get type-checked payloads.
export function useUpdateConfigSection<K extends keyof AllConfig>(section: K) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (body: AllConfig[K]) =>
      api(`/admin/config/${section}`, { method: "PUT", body }),
    onSuccess: () => qc.invalidateQueries({ queryKey: KEY }),
  });
}
