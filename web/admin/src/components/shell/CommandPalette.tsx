import { useNavigate } from "@tanstack/react-router";
import { useEffect, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { STATIC_NAV, usePluginNavGroups } from "./nav.js";

type Entry = { label: string; go: () => void };

export function CommandPalette({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (v: boolean) => void;
}) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const pluginGroups = usePluginNavGroups();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  // Global ⌘K / Ctrl+K toggle.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        onOpenChange(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onOpenChange]);

  useEffect(() => {
    if (open) {
      setQuery("");
      setActive(0);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [open]);

  const entries = useMemo<Entry[]>(() => {
    const items: Entry[] = STATIC_NAV.map((n) => ({
      label: t(n.labelKey),
      go: () => navigate({ to: n.to }),
    }));
    for (const g of pluginGroups) {
      for (const p of g.pages) {
        items.push({
          label: `${g.label} · ${p.title}`,
          go: () =>
            navigate({
              to: "/plugins/$slug/page/$pagePath",
              params: { slug: g.slug, pagePath: p.path },
            }),
        });
      }
    }
    return items;
  }, [t, navigate, pluginGroups]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return entries;
    return entries.filter((e) => e.label.toLowerCase().includes(q));
  }, [entries, query]);

  if (!open) return null;

  const select = (i: number) => {
    const entry = filtered[i];
    if (!entry) return;
    entry.go();
    onOpenChange(false);
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Escape") onOpenChange(false);
    else if (e.key === "ArrowDown") {
      e.preventDefault();
      setActive((a) => Math.min(a + 1, filtered.length - 1));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) => Math.max(a - 1, 0));
    } else if (e.key === "Enter") {
      e.preventDefault();
      select(active);
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/60 pt-[12vh]"
      onClick={() => onOpenChange(false)}
    >
      <div
        className="w-full max-w-[560px] border border-line-strong bg-surface"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={onKeyDown}
      >
        <div className="flex items-center gap-3 border-b border-line px-4 py-3">
          <span className="text-t6">⌕</span>
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => {
              setQuery(e.target.value);
              setActive(0);
            }}
            placeholder="SEARCH ANYTHING…"
            className="w-full bg-transparent text-[13px] tracking-[.5px] text-t1 outline-none placeholder:text-t7"
          />
          <span className="border border-line px-1.5 py-0.5 text-[9px] text-t5">
            ESC
          </span>
        </div>
        <div className="max-h-[50vh] overflow-y-auto py-1">
          {filtered.length === 0 ? (
            <div className="px-4 py-6 text-center text-[11px] uppercase tracking-[2px] text-t7">
              NO MATCHES
            </div>
          ) : (
            filtered.map((e, i) => (
              <button
                key={e.label}
                type="button"
                onMouseEnter={() => setActive(i)}
                onClick={() => select(i)}
                className="flex w-full items-center gap-3 px-4 py-2.5 text-left text-[12px] tracking-[.5px]"
                style={{
                  background: i === active ? "rgba(255,255,255,.04)" : "transparent",
                  color: i === active ? "var(--accent)" : "var(--t3)",
                }}
              >
                <span className="text-t7">▸</span>
                {e.label}
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  );
}
