import { useTelegramAutoLogin } from "../hooks/useTelegramAutoLogin.js";

/**
 * Headless component that drives Telegram Mini App auto-login. Render it once
 * inside the QueryClientProvider (it uses a react-query mutation). Renders
 * nothing; on a successful login the auth store flips to authenticated and the
 * router's guards let the user into the cabinet.
 */
export function TelegramAutoLogin(): null {
  useTelegramAutoLogin();
  return null;
}
