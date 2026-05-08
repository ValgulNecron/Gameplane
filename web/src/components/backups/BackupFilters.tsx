import type { ReactNode } from "react";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import type { GameServer } from "@/types";

interface Props {
  search: string;
  onSearchChange: (v: string) => void;
  server: string;
  onServerChange: (v: string) => void;
  phase: string;
  onPhaseChange: (v: string) => void;
  servers: GameServer[];
  phases: string[];
  trailing?: ReactNode;
}

export function BackupFilters({
  search, onSearchChange,
  server, onServerChange,
  phase, onPhaseChange,
  servers, phases, trailing,
}: Props) {
  return (
    <div className="flex items-center justify-between gap-3">
      <div className="flex flex-1 items-center gap-2">
        <Input
          className="max-w-xs"
          placeholder="Search by name or server…"
          value={search}
          onChange={(e) => onSearchChange(e.target.value)}
        />
        <Select
          className="w-44"
          value={server}
          onValueChange={onServerChange}
          options={[
            { value: "", label: "All servers" },
            ...servers.map((s) => ({ value: s.metadata.name, label: s.metadata.name })),
          ]}
        />
        <Select
          className="w-36"
          value={phase}
          onValueChange={onPhaseChange}
          options={[
            { value: "", label: "All phases" },
            ...phases.map((p) => ({ value: p, label: p })),
          ]}
        />
      </div>
      {trailing && <div className="text-xs text-muted">{trailing}</div>}
    </div>
  );
}
