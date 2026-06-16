export function Segmented<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
}) {
  return (
    <div className="flex gap-px" style={{ background: "var(--line)" }}>
      {options.map((o) => {
        const active = o.value === value;
        return (
          <button
            key={o.value}
            type="button"
            onClick={() => onChange(o.value)}
            className="border-none px-3.5 py-[7px] text-[10px] uppercase tracking-[1px]"
            style={{
              background: active ? "var(--accent)" : "var(--surface)",
              color: active ? "var(--on-accent)" : "var(--t4)",
              fontWeight: active ? 600 : 400,
              cursor: "pointer",
            }}
          >
            {o.label}
          </button>
        );
      })}
    </div>
  );
}
