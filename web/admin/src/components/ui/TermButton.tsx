import type { ButtonHTMLAttributes } from "react";

export function TermButton({
  variant = "primary",
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "ghost" | "danger";
}) {
  const base =
    "inline-flex items-center justify-center gap-2 px-4 py-[9px] text-[11px] font-semibold uppercase tracking-[1px] transition-colors disabled:cursor-not-allowed disabled:opacity-40";
  const styles =
    variant === "primary"
      ? "bg-accent text-on-accent border border-accent hover:opacity-90"
      : variant === "danger"
        ? "border border-danger bg-transparent text-danger hover:bg-danger/10"
        : "border border-line bg-input text-t4 hover:text-t2";
  return <button className={`${base} ${styles} ${className}`} {...props} />;
}
