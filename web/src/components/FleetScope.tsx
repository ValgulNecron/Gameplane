import { ListBox, ListBoxItem, Select } from "@heroui/react";
import type { FleetIssue } from "@/lib/fleet";

export function FleetScopeFilter({ value, onChange, clusters }: { value: string; onChange: (value: string) => void; clusters: string[] }) {
  const choices = [...new Set([...clusters, ...(value ? [value] : [])].filter(Boolean))].sort();
  return <Select value={value} onChange={(next) => onChange(String(next ?? ""))} aria-label="Filter by location" className="w-full max-w-xs">
    <Select.Trigger><Select.Value /><Select.Indicator className="ml-auto h-4 w-4" /></Select.Trigger>
    <Select.Popover><ListBox aria-label="Location options">
      <ListBoxItem id="" textValue="All locations">All locations</ListBoxItem>
      {choices.map((cluster) => <ListBoxItem key={cluster} id={cluster} textValue={cluster}>{cluster}</ListBoxItem>)}
    </ListBox></Select.Popover>
  </Select>;
}

export function FleetCoverage({ partial, issues = [], error, label = "Results" }: { partial?: boolean; issues?: FleetIssue[]; error?: unknown; label?: string }) {
  if (error) return <div role="alert" className="rounded-lg border border-danger/40 bg-danger/5 p-3 text-sm">{label} are unavailable. Try again.</div>;
  if (!partial) return null;
  return <div role="status" className="rounded-lg border border-warning/40 bg-warning/5 p-3 text-sm">
    <strong>{label} are partial.</strong> Counts cover only returned resources.
    {issues.length > 0 && <ul className="mt-1 list-inside list-disc">{issues.map((issue, index) => <li key={`${issue.cluster}/${issue.namespace ?? ""}/${issue.code}/${index}`}>
      {[issue.cluster, issue.namespace].filter(Boolean).join(" / ") || "Selected scope"}: {issue.message || (issue.code === "limit" ? "Result limit reached." : "Unavailable.")}
    </li>)}</ul>}
  </div>;
}
