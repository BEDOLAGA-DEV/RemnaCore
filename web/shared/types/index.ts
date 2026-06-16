// ─── Auth & User ────────────────────────────────────────────────────────────

export const USER_ROLES = {
  customer: "customer",
  reseller: "reseller",
  admin: "admin",
} as const;

export type UserRole = (typeof USER_ROLES)[keyof typeof USER_ROLES];

export type User = {
  id: string;
  email: string;
  display_name: string | null;
  email_verified: boolean;
  role: UserRole;
  telegram_id?: number | null;
  tenant_id?: string | null;
  created_at: string;
  updated_at: string;
};

export type LoginResponse = {
  access_token: string;
  refresh_token: string;
  user: User;
};

export type RegisterResponse = {
  user_id: string;
  email: string;
  verification_token: string;
};

export type SetupStatusResponse = {
  needs_setup: boolean;
};

export type TokenPair = {
  access_token: string;
  refresh_token: string;
};

// ─── Plans ──────────────────────────────────────────────────────────────────

export const BILLING_INTERVALS = {
  month: "month",
  quarter: "quarter",
  year: "year",
} as const;

export type BillingInterval =
  (typeof BILLING_INTERVALS)[keyof typeof BILLING_INTERVALS];

export const PLAN_TIERS = {
  basic: "basic",
  standard: "standard",
  premium: "premium",
} as const;

export type PlanTier = (typeof PLAN_TIERS)[keyof typeof PLAN_TIERS];

export type Plan = {
  id: string;
  group_id?: string;
  name: string;
  description: string | null;
  base_price_amount: number;
  base_price_currency: string;
  billing_interval: BillingInterval;
  traffic_limit_bytes: number;
  device_limit: number;
  allowed_countries: string[];
  allowed_protocols: string[];
  tier: PlanTier;
  max_remnawave_bindings: number;
  family_enabled: boolean;
  max_family_members: number;
  is_active: boolean;
  addons?: PlanAddon[];
  created_at: string;
  updated_at: string;
};

export type PlanAddon = {
  id: string;
  plan_id: string;
  name: string;
  price_amount: number;
  price_currency: string;
  addon_type: string;
  extra_traffic_bytes: number;
  extra_nodes: string[];
  extra_feature_flags: string[];
  created_at: string;
};

// ─── Subscriptions ──────────────────────────────────────────────────────────

export const SUBSCRIPTION_STATUSES = {
  pending: "pending",
  active: "active",
  cancelled: "cancelled",
  expired: "expired",
  paused: "paused",
} as const;

export type SubscriptionStatus =
  (typeof SUBSCRIPTION_STATUSES)[keyof typeof SUBSCRIPTION_STATUSES];

export type Subscription = {
  id: string;
  user_id: string;
  plan_id: string;
  status: SubscriptionStatus;
  period_start: string;
  period_end: string;
  period_interval: string;
  addon_ids: string[];
  assigned_to: string | null;
  cancelled_at: string | null;
  paused_at: string | null;
  created_at: string;
  updated_at: string;
};

// ─── Invoices ───────────────────────────────────────────────────────────────

export const INVOICE_STATUSES = {
  pending: "pending",
  paid: "paid",
  cancelled: "cancelled",
  refunded: "refunded",
} as const;

export type InvoiceStatus =
  (typeof INVOICE_STATUSES)[keyof typeof INVOICE_STATUSES];

export type Invoice = {
  id: string;
  subscription_id: string;
  user_id: string;
  subtotal_amount: number;
  total_discount_amount: number;
  total_amount: number;
  currency: string;
  status: InvoiceStatus;
  paid_at: string | null;
  created_at: string;
  updated_at: string;
};

// ─── Bindings ───────────────────────────────────────────────────────────────

export const BINDING_STATUSES = {
  pending: "pending",
  synced: "synced",
  error: "error",
  disabled: "disabled",
} as const;

export type BindingStatus =
  (typeof BINDING_STATUSES)[keyof typeof BINDING_STATUSES];

export type Binding = {
  id: string;
  subscription_id: string;
  platform_user_id: string;
  remnawave_uuid: string | null;
  remnawave_short_uuid: string | null;
  remnawave_username: string;
  purpose: string;
  status: BindingStatus;
  traffic_limit_bytes: number;
  allowed_nodes: string[];
  inbound_tags: string[];
  synced_at: string | null;
  created_at: string;
  updated_at: string;
};

// ─── Family ─────────────────────────────────────────────────────────────────

export type FamilyGroup = {
  id: string;
  owner_id: string;
  max_members: number;
  members?: FamilyMember[];
  created_at: string;
  updated_at: string;
};

export type FamilyMember = {
  id: string;
  family_group_id: string;
  user_id: string;
  role: string;
  nickname: string | null;
  joined_at: string;
};

// ─── Plugins ────────────────────────────────────────────────────────────────

export const PLUGIN_STATUSES = {
  installed: "installed",
  enabled: "enabled",
  disabled: "disabled",
  error: "error",
} as const;

export type PluginStatus =
  (typeof PLUGIN_STATUSES)[keyof typeof PLUGIN_STATUSES];

export type PluginPageField = {
  key: string;
  label: string;
  type: string; // text, number, boolean, select, multiselect, textarea
  required?: boolean;
  default?: string;
  options?: string[];
  options_url?: string;
  options_value_key?: string;
  options_label_key?: string;
  group?: string;  // Logical section group
  span?: number;   // Grid column span (1 or 2)
};

export type PluginPage = {
  path: string;
  title: string;
  icon: string;
  menu: string;
  collection?: string;
  fields?: PluginPageField[];
  crud_url?: string;
};

export type Plugin = {
  id: string;
  slug: string;
  name: string;
  version: string;
  description: string | null;
  author: string | null;
  license: string | null;
  sdk_version: string | null;
  lang: string | null;
  status: PluginStatus;
  is_builtin: boolean;
  config: Record<string, string>;
  pages: PluginPage[];
  permissions: string[];
  error_log: string | null;
  installed_at: string;
  enabled_at: string | null;
  updated_at: string;
};

export type AggregatedPluginPage = {
  plugin_slug: string;
  plugin_name: string;
  path: string;
  title: string;
  icon: string;
  menu: string;
  collection?: string;
  fields?: PluginPageField[];
  crud_url?: string;
};

// ─── Tenants ────────────────────────────────────────────────────────────────

export type BrandingConfig = {
  logo: string;
  primary_color: string;
  app_name: string;
  support_email: string;
  support_url: string;
};

export type Tenant = {
  id: string;
  name: string;
  domain: string | null;
  owner_user_id: string;
  branding_config: BrandingConfig | null;
  is_active: boolean;
  created_at: string;
  updated_at: string;
};

// ─── Plugin Documents ──────────────────────────────────────────────────

export type PluginDocument = {
  id: string;
  plugin_slug: string;
  collection: string;
  data: Record<string, unknown>;
  created_at: string;
  updated_at: string;
};

// ─── Checkout ───────────────────────────────────────────────────────────────

export type CheckoutResult = {
  subscription_id: string;
  invoice_id: string;
  payment_url: string;
};

// ─── API Error ──────────────────────────────────────────────────────────────

export type ApiError = {
  error: string;
};

// ─── Pagination ─────────────────────────────────────────────────────────────

export type PaginationParams = {
  limit?: number;
  offset?: number;
};

// ─── Admin Metrics & Activity ─────────────────────────────────────────────────

export type AdminMetrics = {
  mrr_cents: number;
  arpu_cents: number;
  churn_30d: number;
  subs_today: number;
  active_subs: number;
  cancelled_subs: number;
  paused_subs: number;
  pending_subs: number;
  total_subs: number;
  total_revenue_cents: number;
  paying_users: number;
};

export type ActivityLevel = "info" | "ok" | "warn";

export type ActivityEntry = {
  id: string;
  timestamp: string;
  event_type: string;
  level: ActivityLevel;
  message: string;
};

export type ActivityListResponse = {
  activity: ActivityEntry[];
};

export type MetricsSample = {
  captured_at: string; // ISO timestamp
  active_users: number;
  active_subs: number;
  mrr_cents: number;
  total_subs: number;
};

export type MetricsHistoryResponse = {
  samples: MetricsSample[];
};

// ─── Remnawave ────────────────────────────────────────────────────────────────

export type RemnawaveOverview = {
  panels_up: number;
  panels_total: number;
  nodes_healthy: number;
  nodes_total: number;
  active_users: number;
  bandwidth: {
    upload: number;
    download: number;
    total: number;
  };
};

export type RealtimeNodeMetrics = {
  uuid: string;
  name: string;
  uploadSpeedBytes: number;
  downloadSpeedBytes: number;
  activeConnections: number;
  collectedAt: string;
};

export type RemnawaveRealtime = {
  panels: Array<{
    panel_id: string;
    slug: string;
    online: boolean;
    metrics: RealtimeNodeMetrics[] | null;
  }>;
};

export type RemnawaveNodeRow = {
  panel_id: string;
  node: {
    uuid: string;
    name: string;
    address: string;
    port: number;
    isConnected: boolean;
    trafficUsedBytes: number;
  };
};

export type NodeBandwidthStats = {
  uuid: string;
  name: string;
  uploadBytes: number;
  downloadBytes: number;
  totalBytes: number;
  date: string;
};

export type TrafficByNode = {
  nodes: Array<{
    panel_id: string;
    stats: NodeBandwidthStats;
  }>;
};
