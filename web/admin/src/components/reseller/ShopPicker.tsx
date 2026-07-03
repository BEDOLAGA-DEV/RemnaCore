import { useMyShops, useShopStore } from "@remnacore/shared";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";

// ShopPicker is the shared active-shop selector for the reseller zone. The
// selected shop id is stored in useShopStore and sent as X-Shop-Id on every
// reseller API request. It auto-selects when the reseller owns exactly one shop.
export function ShopPicker() {
  const { t } = useTranslation();
  const { activeShopId, setActiveShop } = useShopStore();
  const { data: shops } = useMyShops();

  useEffect(() => {
    const first = shops?.[0];
    if (shops?.length === 1 && !activeShopId && first) {
      setActiveShop(first.id);
    }
  }, [shops, activeShopId, setActiveShop]);

  if (!shops || shops.length === 0) return null;

  return (
    <div className="px-2.5 pb-2 pt-1">
      <div className="mb-1 text-[9px] uppercase tracking-[2px] text-t8">
        {t("reseller.bot.shopPicker")}
      </div>
      <select
        value={activeShopId ?? ""}
        onChange={(e) => setActiveShop(e.target.value || null)}
        className="w-full border border-line bg-input px-2 py-1.5 text-[11px] text-t2 outline-none focus:border-accent/45"
      >
        <option value="">{t("reseller.bot.selectShop")}</option>
        {shops.map((s) => (
          <option key={s.id} value={s.id}>
            {s.name}
          </option>
        ))}
      </select>
    </div>
  );
}
