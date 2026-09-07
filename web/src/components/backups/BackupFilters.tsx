import type { ReactNode } from "react";
import {
  Input,
  Select,
  ListBox,
  ListBoxItem,
} from "@heroui/react";
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
          type="text"
        />
        <Select value={server} onChange={(v) => onServerChange(v as string)} className="w-44" aria-label="Filter by server">
          <Select.Trigger>
            <Select.Value />
            <Select.Indicator className="ml-auto h-4 w-4" />
          </Select.Trigger>
          <Select.Popover>
            <ListBox aria-label="Server options">
              <ListBoxItem id="" textValue="All servers">All servers</ListBoxItem>
              {servers.map((s) => (
                <ListBoxItem key={s.metadata.name} id={s.metadata.name} textValue={s.metadata.name}>
                  {s.metadata.name}
                </ListBoxItem>
              ))}
            </ListBox>
          </Select.Popover>
        </Select>
        <Select value={phase} onChange={(v) => onPhaseChange(v as string)} className="w-36" aria-label="Filter by phase">
          <Select.Trigger>
            <Select.Value />
            <Select.Indicator className="ml-auto h-4 w-4" />
          </Select.Trigger>
          <Select.Popover>
            <ListBox aria-label="Phase options">
              <ListBoxItem id="" textValue="All phases">All phases</ListBoxItem>
              {phases.map((p) => (
                <ListBoxItem key={p} id={p} textValue={p}>
                  {p}
                </ListBoxItem>
              ))}
            </ListBox>
          </Select.Popover>
        </Select>
      </div>
      {trailing && <div className="text-xs text-muted">{trailing}</div>}
    </div>
  );
}
