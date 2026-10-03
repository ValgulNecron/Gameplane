import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Templates } from "@/lib/endpoints";
import { assignGameCodesForTemplates } from "@/lib/gameIcon";
import type { GameTemplate } from "@/types";


export function useGameCodes(targetCluster?: string) {
  const clusterId = targetCluster ?? "local";
  const { data: templates } = useQuery({
    queryKey: ["templates", clusterId],
    queryFn: ({ signal }) => Templates.list(clusterId, signal),
  });

  const gameCodes = useMemo(
    () => assignGameCodesForTemplates(templates?.items ?? []),
    [templates],
  );

  const byName = useMemo(() => {
    const m = new Map<string, GameTemplate>();
    for (const t of templates?.items ?? []) {
      m.set(t.metadata.name, t);
    }
    return m;
  }, [templates]);

  return { templates: templates?.items, gameCodes, byName };
}
