// Filled area chart for hero trend panels. Reuses the Sparkline projection
// math but emits a closed, translucent-filled path with the line on top.
// Returns null on <2 points; the caller renders a NO-DATA fallback.
export function AreaChart({
  data,
  color = "var(--accent)",
  w = 600,
  h = 120,
}: {
  data: number[];
  color?: string;
  w?: number;
  h?: number;
}) {
  if (data.length < 2) return null;
  const min = Math.min(...data);
  const max = Math.max(...data);
  const span = max - min || 1;
  const pts = data.map(
    (v, i) => `${(i / (data.length - 1)) * w},${h - ((v - min) / span) * h}`,
  );
  const line = `M${pts.join(" L")}`;
  const area = `${line} L${w},${h} L0,${h} Z`;
  return (
    <svg
      viewBox={`0 0 ${w} ${h}`}
      preserveAspectRatio="none"
      style={{ width: "100%", height: h }}
      aria-hidden
    >
      <path d={area} fill={color} fillOpacity={0.12} stroke="none" />
      <path d={line} fill="none" stroke={color} strokeWidth={1.5} />
    </svg>
  );
}
