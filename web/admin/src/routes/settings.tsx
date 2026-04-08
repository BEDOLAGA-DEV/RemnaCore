import { useState } from "react";
import { useTranslation } from "react-i18next";
import {
  Server,
  Database,
  Key,
  Globe,
  CreditCard,
  MessageSquare,
  Puzzle,
  Activity,
  Gauge,
  Shield,
  Inbox,
  Navigation,
  Flag,
  CircleDot,
  Globe2,
  Signal,
  ChevronDown,
  type LucideIcon,
} from "lucide-react";
import {
  cn,
  LoadingSpinner,
  useAdminSettings,
  type SystemSettings,
} from "@remnacore/shared";

// ─── Setting Row Components ──────────────────────────────────────────────────

type SettingRowProps = {
  label: string;
  value: string | number;
  masked?: boolean;
};

function SettingRow({ label, value, masked = false }: SettingRowProps) {
  const displayValue = String(value);

  return (
    <div className="flex items-start justify-between gap-4 py-2">
      <span className="shrink-0 text-[13px] text-foreground">{label}</span>
      <span
        className={cn(
          "text-right font-mono text-[13px]",
          masked
            ? "italic text-muted-foreground/50"
            : "text-muted-foreground",
        )}
      >
        {displayValue}
      </span>
    </div>
  );
}

type SettingBoolProps = {
  label: string;
  value: boolean;
};

function SettingBool({ label, value }: SettingBoolProps) {
  return (
    <div className="flex items-center justify-between gap-4 py-2">
      <span className="shrink-0 text-[13px] text-foreground">{label}</span>
      <span
        className={cn(
          "rounded-full px-2.5 py-0.5 font-mono text-[11px] font-medium",
          value
            ? "bg-emerald-500/10 text-emerald-500"
            : "bg-red-500/10 text-red-500",
        )}
      >
        {value ? "enabled" : "disabled"}
      </span>
    </div>
  );
}

type SettingArrayProps = {
  label: string;
  values: string[];
};

function SettingArray({ label, values }: SettingArrayProps) {
  return (
    <div className="flex items-start justify-between gap-4 py-2">
      <span className="shrink-0 text-[13px] text-foreground">{label}</span>
      <div className="flex flex-col items-end gap-0.5">
        {values.map((v) => (
          <span
            key={v}
            className="font-mono text-[13px] text-muted-foreground"
          >
            {v}
          </span>
        ))}
      </div>
    </div>
  );
}

// ─── Collapsible Section ─────────────────────────────────────────────────────

type SettingsSectionProps = {
  title: string;
  icon: LucideIcon;
  children: React.ReactNode;
  defaultOpen?: boolean;
};

function SettingsSection({
  title,
  icon: Icon,
  children,
  defaultOpen = true,
}: SettingsSectionProps) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div className="rounded-xl border border-border bg-card">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className="flex w-full items-center gap-3 px-5 py-4 text-left transition-colors hover:bg-secondary/50"
      >
        <Icon size={16} className="shrink-0 text-primary" />
        <span className="flex-1 text-[13px] font-semibold text-foreground">
          {title}
        </span>
        <ChevronDown
          size={14}
          className={cn(
            "shrink-0 text-muted-foreground transition-transform duration-200",
            open && "rotate-180",
          )}
        />
      </button>
      {open ? (
        <div className="border-t border-border px-5 pb-4 pt-2">
          <div className="divide-y divide-border/50">{children}</div>
        </div>
      ) : null}
    </div>
  );
}

// ─── Circuit Breaker Sub-Section ─────────────────────────────────────────────

type CircuitBreakerBlockProps = {
  name: string;
  config: {
    max_failures: number;
    timeout: string;
    max_requests: number;
    interval: string;
  };
};

function CircuitBreakerBlock({ name, config }: CircuitBreakerBlockProps) {
  return (
    <div className="py-2">
      <p className="mb-1 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
        {name}
      </p>
      <div className="grid grid-cols-2 gap-x-6 gap-y-0.5 pl-3">
        <SettingRow label="Max Failures" value={config.max_failures} />
        <SettingRow label="Timeout" value={config.timeout} />
        <SettingRow label="Max Requests" value={config.max_requests} />
        <SettingRow label="Interval" value={config.interval} />
      </div>
    </div>
  );
}

// ─── Settings Content ────────────────────────────────────────────────────────

type SettingsContentProps = {
  settings: SystemSettings;
};

function SettingsContent({ settings }: SettingsContentProps) {
  return (
    <div className="space-y-3">
      {/* Application */}
      <SettingsSection title="Application" icon={Server}>
        <SettingRow label="Port" value={settings.app.port} />
        <SettingRow label="Version" value={settings.app.version} />
        <SettingRow label="Log Level" value={settings.app.log_level} />
        <SettingRow label="Log Format" value={settings.app.log_format} />
      </SettingsSection>

      {/* Database */}
      <SettingsSection title="Database" icon={Database}>
        <SettingRow label="URL" value={settings.database.url} masked />
        <SettingRow label="Max Connections" value={settings.database.max_conns} />
        <SettingRow label="Min Connections" value={settings.database.min_conns} />
        <SettingRow
          label="Max Connection Lifetime"
          value={settings.database.max_conn_lifetime}
        />
        <SettingRow
          label="Max Connection Idle Time"
          value={settings.database.max_conn_idle_time}
        />
        <SettingRow
          label="Health Check Period"
          value={settings.database.health_check_period}
        />
      </SettingsSection>

      {/* Cache */}
      <SettingsSection title="Cache (Valkey)" icon={Database}>
        <SettingRow label="URL" value={settings.cache.url} masked />
      </SettingsSection>

      {/* Message Queue */}
      <SettingsSection title="Message Queue (NATS)" icon={Inbox}>
        <SettingRow label="URL" value={settings.message_queue.url} masked />
      </SettingsSection>

      {/* JWT Authentication */}
      <SettingsSection title="JWT Authentication" icon={Key}>
        <SettingRow
          label="Private Key Path"
          value={settings.jwt.private_key_path}
        />
        <SettingRow
          label="Public Key Path"
          value={settings.jwt.public_key_path}
        />
        <SettingRow
          label="Access Token TTL"
          value={settings.jwt.access_token_ttl}
        />
        <SettingRow
          label="Refresh Token TTL"
          value={settings.jwt.refresh_token_ttl}
        />
      </SettingsSection>

      {/* Remnawave */}
      <SettingsSection title="Remnawave" icon={Globe}>
        <SettingRow label="URL" value={settings.remnawave.url} />
        <SettingRow
          label="API Token"
          value={settings.remnawave.api_token}
          masked
        />
        <SettingRow
          label="Webhook Secret"
          value={settings.remnawave.webhook_secret}
          masked
        />
      </SettingsSection>

      {/* Billing */}
      <SettingsSection title="Billing" icon={CreditCard}>
        <SettingRow label="Trial Days" value={settings.billing.trial_days} />
      </SettingsSection>

      {/* Telegram */}
      <SettingsSection title="Telegram" icon={MessageSquare}>
        <SettingRow
          label="Bot Token"
          value={settings.telegram.bot_token}
          masked
        />
        <SettingRow
          label="Webhook URL"
          value={settings.telegram.webhook_url}
        />
        <SettingRow
          label="Cabinet URL"
          value={settings.telegram.cabinet_url}
        />
      </SettingsSection>

      {/* Plugins */}
      <SettingsSection title="Plugins" icon={Puzzle}>
        <SettingRow label="Directory" value={settings.plugins.plugins_dir} />
        <SettingRow label="Max Plugins" value={settings.plugins.max_plugins} />
        <SettingBool label="Hot Reload" value={settings.plugins.enable_hot_reload} />
      </SettingsSection>

      {/* Infrastructure */}
      <SettingsSection title="Infrastructure" icon={Activity}>
        <SettingRow
          label="Health Check Interval"
          value={settings.infrastructure.health_check_interval}
        />
        <SettingRow
          label="Max Concurrent Checks"
          value={settings.infrastructure.max_concurrent_checks}
        />
        <SettingRow
          label="Speed Test Port"
          value={settings.infrastructure.speed_test_port}
        />
        <SettingRow
          label="Subscription Proxy Port"
          value={settings.infrastructure.subscription_proxy_port}
        />
      </SettingsSection>

      {/* Speed Test */}
      <SettingsSection title="Speed Test" icon={Gauge}>
        <SettingRow
          label="Max Concurrent"
          value={settings.speed_test.max_concurrent}
        />
        <SettingRow
          label="Per-IP Rate Limit"
          value={settings.speed_test.per_ip_rate_limit}
        />
        <SettingRow
          label="Max Upload Bytes"
          value={settings.speed_test.max_upload_bytes.toLocaleString()}
        />
      </SettingsSection>

      {/* Rate Limiting */}
      <SettingsSection title="Rate Limiting" icon={Shield}>
        <SettingRow
          label="Checkout Max / Hour"
          value={settings.rate_limit.checkout_max_per_hour}
        />
        <SettingRow
          label="Subscription Max / Day"
          value={settings.rate_limit.subscription_max_per_day}
        />
        <SettingRow
          label="Login Max / Window"
          value={settings.rate_limit.login_max_per_window}
        />
        <SettingRow
          label="Login Window (min)"
          value={settings.rate_limit.login_window_minutes}
        />
        <SettingRow
          label="Forgot Password Max / Window"
          value={settings.rate_limit.forgot_pwd_max_per_window}
        />
        <SettingRow
          label="Forgot Password Window (min)"
          value={settings.rate_limit.forgot_pwd_window_minutes}
        />
      </SettingsSection>

      {/* Outbox */}
      <SettingsSection title="Outbox Relay" icon={Inbox}>
        <SettingRow
          label="Relay Workers"
          value={settings.outbox.relay_workers}
        />
        <SettingRow
          label="Partition Lookahead"
          value={settings.outbox.partition_lookahead}
        />
        <SettingRow
          label="Retention Days"
          value={settings.outbox.retention_days}
        />
      </SettingsSection>

      {/* Smart Router */}
      <SettingsSection title="Smart Router" icon={Navigation}>
        <p className="pb-1 pt-2 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
          Default Weights
        </p>
        <SettingRow label="Geo" value={settings.smart_router.weight_geo} />
        <SettingRow
          label="Latency"
          value={settings.smart_router.weight_latency}
        />
        <SettingRow label="Load" value={settings.smart_router.weight_load} />
        <p className="pb-1 pt-3 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
          Gaming Weights
        </p>
        <SettingRow
          label="Geo"
          value={settings.smart_router.weight_gaming_geo}
        />
        <SettingRow
          label="Latency"
          value={settings.smart_router.weight_gaming_latency}
        />
        <SettingRow
          label="Load"
          value={settings.smart_router.weight_gaming_load}
        />
        <p className="pb-1 pt-3 text-[12px] font-medium uppercase tracking-wider text-muted-foreground">
          Streaming Weights
        </p>
        <SettingRow
          label="Geo"
          value={settings.smart_router.weight_streaming_geo}
        />
        <SettingRow
          label="Latency"
          value={settings.smart_router.weight_streaming_latency}
        />
        <SettingRow
          label="Load"
          value={settings.smart_router.weight_streaming_load}
        />
      </SettingsSection>

      {/* Feature Flags */}
      <SettingsSection title="Feature Flags" icon={Flag}>
        <SettingBool
          label="Subscription Hooks"
          value={settings.feature_flags.hooks_subscription_enabled}
        />
        <SettingBool
          label="VPN Provider Hooks"
          value={settings.feature_flags.hooks_vpn_provider_enabled}
        />
      </SettingsSection>

      {/* Circuit Breakers */}
      <SettingsSection title="Circuit Breakers" icon={CircleDot}>
        <CircuitBreakerBlock
          name="Remnawave"
          config={settings.circuit_breaker.remnawave}
        />
        <CircuitBreakerBlock
          name="Outbox NATS"
          config={settings.circuit_breaker.outbox_nats}
        />
        <CircuitBreakerBlock
          name="Valkey"
          config={settings.circuit_breaker.valkey}
        />
        <CircuitBreakerBlock
          name="VPN Provider"
          config={settings.circuit_breaker.vpn_provider}
        />
      </SettingsSection>

      {/* CORS */}
      <SettingsSection title="CORS" icon={Globe2}>
        <SettingArray
          label="Allowed Origins"
          values={settings.cors.allowed_origins}
        />
      </SettingsSection>

      {/* Tracing */}
      <SettingsSection title="Tracing" icon={Signal}>
        <SettingRow label="Endpoint" value={settings.tracing.endpoint} />
      </SettingsSection>
    </div>
  );
}

// ─── Page ────────────────────────────────────────────────────────────────────

export function SettingsPage() {
  const { t } = useTranslation();
  const { data: settings, isLoading, isError, error } = useAdminSettings();

  if (isLoading) {
    return <LoadingSpinner />;
  }

  if (isError) {
    return (
      <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-border p-12">
        <p className="text-[13px] text-destructive">
          {t("common.errorLoading")}:{" "}
          {error instanceof Error ? error.message : t("common.unknownError")}
        </p>
      </div>
    );
  }

  if (!settings) {
    return null;
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-[15px] font-semibold text-foreground">
          System Settings
        </h1>
        <div className="flex items-center gap-2">
          <span className="rounded-full bg-primary/10 px-2.5 py-0.5 font-mono text-[11px] font-medium text-primary">
            v{settings.app.version}
          </span>
          <span className="rounded-full bg-secondary px-2.5 py-0.5 font-mono text-[11px] font-medium text-muted-foreground">
            :{settings.app.port}
          </span>
        </div>
      </div>

      <SettingsContent settings={settings} />
    </div>
  );
}
