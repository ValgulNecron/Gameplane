import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { Templates } from "@/lib/endpoints";
import { assignGameCodesForTemplates } from "@/lib/gameIcon";
import type { GameTemplate } from "@/types";

export function useGameCodes() {
  const { data: templates } = useQuery({
    queryKey: ["templates"],
    queryFn: () => Templates.list(),
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
