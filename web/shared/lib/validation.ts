import { z } from "zod";

/**
 * Minimum password length — must match the Go backend constant
 * `identity.MinPasswordLength` (internal/domain/identity/model.go).
 */
const MIN_PASSWORD_LENGTH = 8;

/**
 * Password schema that mirrors the backend `validatePassword` rules:
 * - At least 8 characters
 * - At least one uppercase letter
 * - At least one lowercase letter
 * - At least one digit
 *
 * Reuse this in every form that accepts a password (register, reset-password).
 */
export const passwordSchema = z
  .string()
  .min(MIN_PASSWORD_LENGTH, "validation.passwordMinLength")
  .regex(/[A-Z]/, "validation.passwordUppercase")
  .regex(/[a-z]/, "validation.passwordLowercase")
  .regex(/[0-9]/, "validation.passwordDigit");

/**
 * Email schema — the single source of email validation.
 *
 * Reuse this in every form that accepts an email (login, register, setup,
 * forgot-password, tenant branding).
 */
export const emailSchema = z.email();

/**
 * HTTPS-only URL schema, mirroring the backend rule in
 * `ResellerService.SetShopBot` (strings.HasPrefix(url, "https://")): a valid
 * URL whose raw string literally starts with "https://".
 *
 * Reuse this wherever the backend requires an https URL (shop cabinet URL).
 */
export const httpsURLSchema = z.url().startsWith("https://");
