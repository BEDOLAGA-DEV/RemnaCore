import { create } from "zustand";
import { persist } from "zustand/middleware";
import { SHOP_STORAGE_KEY } from "../lib/constants.js";

type ShopState = {
  /** Active shop (tenant) id sent as X-Shop-Id on API requests; null = none. */
  activeShopId: string | null;
};

type ShopActions = {
  setActiveShop: (shopId: string | null) => void;
};

/**
 * Persisted active-shop context for shop-scoped (reseller) API calls. The
 * backend independently verifies membership per request (ShopResolver), so
 * this is a convenience header source, never a trust boundary.
 */
export const useShopStore = create<ShopState & ShopActions>()(
  persist(
    (set) => ({
      activeShopId: null,

      setActiveShop: (shopId) => set({ activeShopId: shopId }),
    }),
    {
      name: SHOP_STORAGE_KEY,
    },
  ),
);
