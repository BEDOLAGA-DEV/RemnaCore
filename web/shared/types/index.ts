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

export type TokenPair = {
  access_token: string;
  refresh_token: string;
};

// ─── Plans ──────────────────────────────────────────────────────────────────

export const BILLING_INTERVALS = {
  monthly: "monthly",
  quarterly: "quarterly",
  yearly: "yearly",
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
  config: Record<string, string>;
  permissions: string[];
  error_log: string | null;
  installed_at: string;
  enabled_at: string | null;
  updated_at: string;
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
