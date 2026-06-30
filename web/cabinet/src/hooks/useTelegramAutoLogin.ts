import { useAuthStore, useTelegramWebAppLogin } from "@remnacore/shared";
import { useEffect, useRef } from "react";

/**
 * When the cabinet is opened as a Telegram Mini App, silently authenticate the
 * user from `window.Telegram.WebApp.initData`: POST it to the backend (which
 * resolves the shop from the request domain and validates the initData HMAC),
 * and store the returned session. Outside Telegram, or when already
 * authenticated, this is a no-op. It attempts at most once per mount.
 */
export function useTelegramAutoLogin(): void {
  const { isAuthenticated } = useAuthStore();
  const webAppLogin = useTelegramWebAppLogin();
  const attempted = useRef(false);

  useEffect(() => {
    if (attempted.current || isAuthenticated) {
      return;
    }
    const webApp = window.Telegram?.WebApp;
    if (!webApp) {
      return;
    }
    webApp.ready();
    if (!webApp.initData) {
      return;
    }
    attempted.current = true;
    webAppLogin.mutate({ init_data: webApp.initData });
  }, [isAuthenticated, webAppLogin]);
}
