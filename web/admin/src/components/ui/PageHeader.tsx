import type { ReactNode } from "react";

export function PageHeader({
  title,
  breadcrumb,
  right,
}: {
  title: string;
  breadcrumb?: string;
  right?: ReactNode;
}) {
  return (
    <div className="mb-3.5 flex items-end justify-between gap-4">
      <div>
        <h1 className="m-0 text-[26px] font-extrabold uppercase tracking-[1px]">
          {title}
          <span
            className="text-accent"
            style={{ animation: "blink 1.1s step-end infinite" }}
          >
            _
          </span>
        </h1>
        {breadcrumb && (
          <div className="mt-1 text-[10px] uppercase tracking-[2px] text-t7">
            {breadcrumb}
          </div>
        )}
      </div>
      {right}
    </div>
  );
}
