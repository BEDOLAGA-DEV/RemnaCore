import type { ReactNode } from "react";

export function KpiGrid({
  cols,
  children,
}: {
  cols: number;
  children: ReactNode;
}) {
  return (
    <div
      className="grid border border-line"
      style={{
        gridTemplateColumns: `repeat(${cols},1fr)`,
        gap: "1px",
        background: "var(--line)",
      }}
    >
      {children}
    </div>
  );
}

export function StatCell({
  label,
  value,
  foot,
  dot = "var(--accent)",
}: {
  label: string;
  value: ReactNode;
  foot?: ReactNode;
  dot?: string;
}) {
  return (
    <div className="flex flex-col gap-2.5 bg-surface p-4">
      <div className="flex items-center justify-between">
        <span className="text-[10px] uppercase tracking-[1.5px] text-t6">
          {label}
        </span>
        <span className="h-[5px] w-[5px] rounded-full" style={{ background: dot }} />
      </div>
      <div className="text-[30px] font-bold leading-none tracking-[.5px] tabular-nums">
        {value}
      </div>
      {foot && (
        <div className="flex items-center justify-between gap-2">{foot}</div>
      )}
    </div>
  );
}
