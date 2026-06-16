import type { ReactNode } from "react";

export type Column<T> = {
  key: string;
  header: string;
  align?: "left" | "right" | "center";
  render: (row: T) => ReactNode;
};

export function DataTable<T>({
  columns,
  rows,
  cols,
  rowKey,
  onRowClick,
  empty = "NO DATA",
}: {
  columns: Column<T>[];
  rows: T[];
  /** CSS grid-template-columns value, e.g. "1.6fr .8fr 1fr 40px" */
  cols: string;
  rowKey: (r: T) => string;
  onRowClick?: (r: T) => void;
  empty?: string;
}) {
  const grid = { gridTemplateColumns: cols, gap: "14px" } as const;
  return (
    <section className="border border-line bg-surface">
      <div
        className="grid border-b border-line px-4 py-[11px] text-[9px] uppercase tracking-[1.5px] text-t8"
        style={grid}
      >
        {columns.map((c) => (
          <span key={c.key} style={{ textAlign: c.align ?? "left" }}>
            {c.header}
          </span>
        ))}
      </div>
      {rows.length === 0 ? (
        <div className="px-4 py-10 text-center text-[11px] uppercase tracking-[2px] text-t7">
          {empty}
        </div>
      ) : (
        rows.map((r, i) => (
          <div
            key={rowKey(r)}
            onClick={onRowClick ? () => onRowClick(r) : undefined}
            className="grid items-center border-b px-4 py-3 text-[12px] hover:bg-white/[.025]"
            style={{
              ...grid,
              borderColor: "var(--line-soft)",
              animation: `rowin .25s ease ${Math.min(i, 12) * 0.02}s both`,
              cursor: onRowClick ? "pointer" : "default",
            }}
          >
            {columns.map((c) => (
              <span key={c.key} style={{ textAlign: c.align ?? "left" }}>
                {c.render(r)}
              </span>
            ))}
          </div>
        ))
      )}
    </section>
  );
}
