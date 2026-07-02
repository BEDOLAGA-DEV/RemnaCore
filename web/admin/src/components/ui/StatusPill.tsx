export type Tone = "ok" | "warn" | "danger" | "muted";

const TONE: Record<Tone, string> = {
  ok: "var(--accent)",
  warn: "var(--warn)",
  danger: "var(--danger)",
  muted: "var(--t6)",
};

export function StatusPill({
  label,
  tone = "ok",
}: {
  label: string;
  tone?: Tone;
}) {
  const c = TONE[tone];
  return (
    <span
      className="inline-flex items-center gap-1.5 border px-2 py-[3px] text-[10px] uppercase tracking-[1px]"
      style={{ borderColor: c, color: c }}
    >
      <span
        className="h-[5px] w-[5px] rounded-full"
        style={{ background: c }}
      />
      {label}
    </span>
  );
}
