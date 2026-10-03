import { useQuery } from "@tanstack/react-query";
import { createRequestClient } from "./api";
import { resourceKey, type ResourceTarget } from "./resourceTarget";

export interface CaptureCapabilities {
  enabled: boolean;
  files: boolean;
  state: "ready" | "unsupported" | "unavailable";
  defaultRetentionSeconds: number;
  maxRetentionSeconds: number;
  defaultMaxDurationSeconds: number;
  defaultMaxSizeBytes: number;
}

interface ServerCapabilities {
  target: ResourceTarget;
  capture: CaptureCapabilities;
}

export function useServerCapabilities(target: ResourceTarget, enabled: boolean) {
  return useQuery({
    queryKey: resourceKey(target, "capabilities"),
    queryFn: async ({ signal }) => {
      const result = await createRequestClient(target, signal).api<ServerCapabilities>(
        `/servers/${encodeURIComponent(target.name)}/capabilities`,
      );
      if (result.target.cluster !== target.cluster || result.target.namespace !== target.namespace ||
          result.target.name !== target.name || (target.uid && result.target.uid !== target.uid)) {
        throw new Error("The server identity changed. Reload this server before continuing.");
      }
      const capture = result.capture;
      if (capture.state === "ready" && (
        ![capture.defaultRetentionSeconds, capture.maxRetentionSeconds, capture.defaultMaxDurationSeconds, capture.defaultMaxSizeBytes]
          .every((value) => Number.isSafeInteger(value) && value > 0) ||
        capture.defaultRetentionSeconds < 60 || capture.defaultRetentionSeconds > capture.maxRetentionSeconds || capture.defaultMaxDurationSeconds > 3600
      )) {
        throw new Error("Capture limits are unavailable for this location.");
      }
      return result.capture;
    },
    enabled,
    retry: false,
    refetchInterval: 30_000,
  });
}
