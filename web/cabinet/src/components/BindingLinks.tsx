import { useTranslation } from "react-i18next";
import { Copy, Check, Wifi, WifiOff } from "lucide-react";
import { useState } from "react";
import { cn, formatBytes } from "@remnacore/shared";
import type { Binding, BindingStatus } from "@remnacore/shared";

type BindingLinksProps = {
  bindings: Binding[];
};

function statusIcon(status: BindingStatus) {
  if (status === "synced") return <Wifi size={14} className="text-green-500" />;
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
      className="rounded-md p-1.5 text-muted-foreground hover:bg-accent hover:text-foreground transition-colors"
      aria-label={t("common.copyToClipboard")}
    >
      {copied ? (
        <Check size={14} className="text-green-500" />
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
      {bindings.map((binding) => (
        <div
          key={binding.id}
          className="rounded-lg border border-border bg-card p-4"
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
                    ? "bg-green-500/10 text-green-500"
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

          <div className="mt-2 text-xs text-muted-foreground">
            {t("bindings.trafficUsed")}:{" "}
            {formatBytes(binding.traffic_limit_bytes)}
          </div>
        </div>
      ))}
    </div>
  );
}
