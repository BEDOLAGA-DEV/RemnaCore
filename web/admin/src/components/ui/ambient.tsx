// Ambient terminal decorations: grid backdrop, scan line, pulse dot.

export function GridBackdrop() {
  return (
    <div
      aria-hidden
      className="pointer-events-none absolute inset-0"
      style={{
        backgroundImage:
          "linear-gradient(rgba(255,255,255,.025) 1px,transparent 1px),linear-gradient(90deg,rgba(255,255,255,.025) 1px,transparent 1px)",
        backgroundSize: "42px 42px",
      }}
    />
  );
}

export function ScanLine() {
  return (
    <div
      aria-hidden
      className="pointer-events-none absolute left-0 right-0 top-0 h-3.5"
      style={{
        background:
          "linear-gradient(180deg,color-mix(in srgb,var(--accent) 22%,transparent),transparent)",
        animation: "scan 5s linear infinite",
      }}
    />
  );
}

export function PulseDot({ color = "var(--accent)" }: { color?: string }) {
  return (
    <span className="relative inline-flex h-[7px] w-[7px]">
      <span
        className="h-[7px] w-[7px] rounded-full"
        style={{ background: color }}
      />
      <span
        className="absolute inset-0 rounded-full"
        style={{ background: color, animation: "pulse-ring 1.8s infinite" }}
      />
    </span>
  );
}
