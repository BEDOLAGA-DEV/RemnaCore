/**
 * API-specific request/response types that don't belong in domain types.
 */

// ─── Auth Requests ──────────────────────────────────────────────────────────

export type LoginRequest = {
  email: string;
  password: string;
};

export type RegisterRequest = {
  email: string;
  password: string;
};

export type VerifyEmailRequest = {
  token: string;
};

export type RefreshTokenRequest = {
  refresh_token: string;
};

export type ForgotPasswordRequest = {
  email: string;
};

export type ResetPasswordRequest = {
  token: string;
  new_password: string;
};

// ─── Profile Requests ───────────────────────────────────────────────────────

export type UpdateProfileRequest = {
  display_name: string;
};

export type LinkTelegramRequest = {
  telegram_id: number;
};

// ─── Billing Requests ───────────────────────────────────────────────────────

export type CreateSubscriptionRequest = {
  plan_id: string;
  addon_ids?: string[];
};

export type StartCheckoutRequest = {
  plan_id: string;
  addon_ids?: string[];
  return_url: string;
  cancel_url: string;
};

export type AddAddonRequest = {
  addon_id: string;
};

// ─── Family Requests ────────────────────────────────────────────────────────

export type CreateFamilyRequest = {
  subscription_id: string;
};

export type AddFamilyMemberRequest = {
  subscription_id: string;
  member_user_id: string;
  nickname?: string;
};

// ─── Admin Requests ─────────────────────────────────────────────────────────

export type InstallPluginRequest = {
  manifest: string;
  wasm: string;
};

export type UpdatePluginConfigRequest = {
  config: Record<string, string>;
};

export type CreateTenantRequest = {
  name: string;
  domain?: string;
  owner_user_id: string;
};

export type UpdateBrandingRequest = {
  logo: string;
  primary_color: string;
  app_name: string;
  support_email: string;
  support_url: string;
};

// ─── Generic status response ────────────────────────────────────────────────

export type StatusResponse = {
  status: string;
};

// ─── Create subscription response ───────────────────────────────────────────

import type { Invoice, Subscription } from "../types/index.js";

export type CreateSubscriptionResponse = {
  subscription: Subscription;
  invoice: Invoice;
};

// ─── Create tenant response ─────────────────────────────────────────────────

import type { Tenant } from "../types/index.js";

export type CreateTenantResponse = {
  tenant: Tenant;
  api_key: string;
};
