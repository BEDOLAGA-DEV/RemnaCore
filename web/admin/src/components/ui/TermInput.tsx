import type { InputHTMLAttributes } from "react";

export function TermInput({
  label,
  error,
  className = "",
  ...props
}: InputHTMLAttributes<HTMLInputElement> & {
  label?: string;
  error?: string;
}) {
  return (
    <div>
      {label && (
        <div className="mb-2 text-[9px] uppercase tracking-[1.5px] text-t6">
          {label}
        </div>
      )}
      <input
        className={`w-full border bg-input px-3 py-3 text-[13px] tracking-[.5px] text-t1 outline-none placeholder:text-t7 focus:border-accent/45 ${className}`}
        style={{ borderColor: error ? "var(--danger)" : "var(--line-strong)" }}
        {...props}
      />
      {error && (
        <div className="mt-1 text-[9px] uppercase tracking-[1px] text-danger">
          {error}
        </div>
      )}
    </div>
  );
}
