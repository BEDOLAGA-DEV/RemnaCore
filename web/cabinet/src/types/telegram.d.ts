// Minimal typing for the Telegram Mini App SDK injected by
// https://telegram.org/js/telegram-web-app.js (see index.html). Only the
// surface the cabinet uses is declared.
export {};

declare global {
  interface TelegramWebApp {
    /** Raw initData query string to validate server-side (empty outside Telegram). */
    readonly initData: string;
    /** Signals to Telegram that the Mini App is ready to be displayed. */
    ready: () => void;
  }

  interface Window {
    Telegram?: {
      WebApp?: TelegramWebApp;
    };
  }
}
