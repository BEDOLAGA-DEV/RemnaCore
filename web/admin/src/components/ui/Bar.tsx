export function Bar({
  pct,
  color = "var(--accent)",
}: {
  pct: number;
  color?: string;
}) {
  const w = Math.min(100, Math.max(0, pct));
  return (
    <span
      className="block h-[5px] flex-1"
      style={{ background: "rgba(255,255,255,.07)" }}
    >
      <span className="block h-full" style={{ width: `${w}%`, background: color }} />
    </span>
  );
}
