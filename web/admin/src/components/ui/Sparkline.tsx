export function Sparkline({
  data,
  color = "var(--accent)",
  w = 110,
  h = 30,
}: {
  data: number[];
  color?: string;
  w?: number;
  h?: number;
}) {
  if (data.length < 2) return <svg width={w} height={h} aria-hidden />;
  const min = Math.min(...data);
  const max = Math.max(...data);
  const span = max - min || 1;
  const pts = data.map(
    (v, i) =>
      `${(i / (data.length - 1)) * w},${h - ((v - min) / span) * h}`,
  );
  return (
    <svg
      viewBox={`0 0 ${w} ${h}`}
      preserveAspectRatio="none"
      style={{ width: w, height: h }}
      aria-hidden
    >
      <path d={`M${pts.join(" L")}`} fill="none" stroke={color} strokeWidth={1.5} />
    </svg>
  );
}
