import type { ReactNode } from "react";
import { ScanLine } from "./ambient.js";

export function Panel({
  children,
  scanline = false,
  className = "",
}: {
  children: ReactNode;
  scanline?: boolean;
  className?: string;
}) {
  return (
    <section
      className={`relative overflow-hidden border border-line bg-surface ${className}`}
    >
      {scanline && <ScanLine />}
      {children}
    </section>
  );
}

export function PanelHeader({
  title,
  right,
}: {
  title: string;
  right?: ReactNode;
}) {
  return (
    <div className="flex items-center justify-between border-b border-line px-4 py-3">
      <div className="text-[11px] uppercase tracking-[2px] text-t6">
        {title}
      </div>
      {right}
    </div>
  );
}
