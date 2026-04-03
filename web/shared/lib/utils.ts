import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";
import { BYTES_PER_KB, CENTS_PER_UNIT } from "./constants.js";

/**
 * Merge Tailwind classes with clsx. Use everywhere for conditional classes.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}

/**
 * Format bytes to a human-readable string.
 */
export function formatBytes(bytes: number, decimals = 2): string {
  if (bytes === 0) return "0 B";

  const dm = decimals < 0 ? 0 : decimals;
  const sizes = ["B", "KB", "MB", "GB", "TB"];

  const i = Math.floor(Math.log(bytes) / Math.log(BYTES_PER_KB));
  const size = sizes[i] ?? "TB";
  return `${Number.parseFloat((bytes / BYTES_PER_KB ** i).toFixed(dm))} ${size}`;
}

/**
 * Format amount in minor units (cents) to a currency string.
 */
export function formatMoney(
  amount: number,
  currency = "USD",
  locale = "en-US",
): string {
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    minimumFractionDigits: 2,
  }).format(amount / CENTS_PER_UNIT);
}

/**
 * Format ISO date string to a localized short date.
 */
export function formatDate(
  date: string | null | undefined,
  locale = "en-US",
): string {
  if (!date) return "—";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "short",
    day: "numeric",
  }).format(new Date(date));
}

/**
 * Format ISO date string to localized date + time.
 */
export function formatDateTime(
  date: string | null | undefined,
  locale = "en-US",
): string {
  if (!date) return "—";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(date));
}
