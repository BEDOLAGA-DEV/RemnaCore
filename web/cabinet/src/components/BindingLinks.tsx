import { useTranslation } from "react-i18next";
import { Copy, Check, Wifi, WifiOff } from "lucide-react";
import { useState } from "react";
import { cn, formatBytes } from "@remnacore/shared";
import type { Binding, BindingStatus } from "@remnacore/shared";

type BindingLinksProps = {
  bindings: Binding[];
};

function statusIcon(status: BindingStatus) {
  if (status === "synced") return <Wifi size={14} className="text-[#2dd4bf]" />;
  if (status === "error") return <WifiOff size={14} className="text-red-500" />;
  return <Wifi size={14} className="text-muted-foreground" />;
}

function CopyButton({ text }: { text: string }) {
  const { t } = useTranslation();
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <button
      type="button"
      onClick={handleCopy}
      className="rounded-[8px] p-1.5 text-muted-foreground transition-all hover:bg-[rgba(245,236,227,0.06)] hover:text-foreground"
      aria-label={t("common.copyToClipboard")}
    >
      {copied ? (
        <Check size={14} className="text-[#2dd4bf]" />
      ) : (
        <Copy size={14} />
      )}
    </button>
  );
}

export function BindingLinks({ bindings }: BindingLinksProps) {
  const { t } = useTranslation();

  if (bindings.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">{t("bindings.empty")}</p>
    );
  }

  return (
    <div className="flex flex-col gap-3">
      {bindings.map((binding, index) => (
        <div
          key={binding.id}
          className={cn(
            "animate-fade-up rounded-[10px] border border-border bg-card p-4 transition-all",
            binding.status === "synced" &&
              "bg-[rgba(45,212,191,0.04)] border-[rgba(45,212,191,0.08)]",
          )}
          style={{ animationDelay: `${index * 50}ms` }}
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              {statusIcon(binding.status)}
              <span className="font-mono text-sm text-foreground">
                {binding.remnawave_username}
              </span>
              <span
                className={cn(
                  "rounded-full px-2 py-0.5 text-xs font-medium",
                  binding.status === "synced"
                    ? "bg-[rgba(45,212,191,0.10)] text-[#2dd4bf]"
                    : binding.status === "error"
                      ? "bg-red-500/10 text-red-500"
                      : "bg-muted text-muted-foreground",
                )}
              >
                {t(`bindings.status.${binding.status}`)}
              </span>
            </div>
            {binding.remnawave_short_uuid && (
              <CopyButton text={binding.remnawave_short_uuid} />
            )}
          </div>

          <div className="mt-2 font-mono text-xs text-muted-foreground">
            {t("bindings.trafficUsed")}:{" "}
            {formatBytes(binding.traffic_limit_bytes)}
          </div>
        </div>
      ))}
    </div>
  );
}
