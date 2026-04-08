import { useQuery } from "@tanstack/react-query";
import { apiGet } from "../client.js";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";

type CircuitBreakerConfig = {
  max_failures: number;
  timeout: string;
  max_requests: number;
  interval: string;
};

export type SystemSettings = {
  app: {
    port: number;
    version: string;
    log_level: string;
    log_format: string;
  };
  database: {
    url: string;
    max_conns: number;
    min_conns: number;
    max_conn_lifetime: string;
    max_conn_idle_time: string;
    health_check_period: string;
  };
  cache: {
    url: string;
  };
  message_queue: {
    url: string;
  };
  jwt: {
    private_key_path: string;
    public_key_path: string;
    access_token_ttl: string;
    refresh_token_ttl: string;
  };
  remnawave: {
    url: string;
    api_token: string;
    webhook_secret: string;
  };
  billing: {
    trial_days: number;
  };
  telegram: {
    bot_token: string;
    webhook_url: string;
    cabinet_url: string;
  };
  plugins: {
    dir: string;
    max_plugins: number;
    hot_reload: boolean;
  };
  infrastructure: {
    health_check_interval: string;
    max_concurrent_checks: number;
    speed_test_port: number;
    subscription_proxy_port: number;
  };
  speed_test: {
    max_concurrent: number;
    per_ip_rate_limit: number;
    max_upload_bytes: number;
  };
  rate_limit: {
    checkout_max_per_hour: number;
    subscription_max_per_day: number;
    login_max_per_window: number;
    login_window_minutes: number;
    forgot_pwd_max_per_window: number;
    forgot_pwd_window_minutes: number;
  };
  outbox: {
    relay_workers: number;
    partition_lookahead: number;
    retention_days: number;
  };
  smart_router: {
    weight_geo: number;
    weight_latency: number;
    weight_load: number;
    weight_gaming_geo: number;
    weight_gaming_latency: number;
    weight_gaming_load: number;
    weight_streaming_geo: number;
    weight_streaming_latency: number;
    weight_streaming_load: number;
  };
  feature_flags: {
    hooks_subscription_enabled: boolean;
    hooks_vpn_provider_enabled: boolean;
  };
  circuit_breaker: {
    remnawave: CircuitBreakerConfig;
    outbox_nats: CircuitBreakerConfig;
    valkey: CircuitBreakerConfig;
    vpn_provider: CircuitBreakerConfig;
  };
  cors: {
    allowed_origins: string[];
  };
  tracing: {
    endpoint: string;
  };
};

const SETTINGS_STALE_TIME_MS = 60_000;

export function useAdminSettings() {
  return useQuery({
    queryKey: QUERY_KEYS.admin.settings,
    queryFn: () => apiGet<SystemSettings>(ENDPOINTS.admin.settings),
    staleTime: SETTINGS_STALE_TIME_MS,
  });
}
