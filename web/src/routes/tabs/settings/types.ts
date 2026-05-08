import type { GameServer, GameTemplate } from "@/types";

export interface SectionProps {
  draft: GameServer;
  onChange: (next: GameServer) => void;
  template?: GameTemplate;
}

export const DESCRIPTION_ANNOTATION = "kestrel.gg/description";
export const GRACE_PERIOD_ANNOTATION = "kestrel.gg/grace-period-seconds";
