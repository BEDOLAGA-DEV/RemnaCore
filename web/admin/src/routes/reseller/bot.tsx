import {
  LoadingSpinner,
  useMyShops,
  useResellerBot,
  useResellerBotPlugins,
  useShopStore,
  useUpdateResellerBot,
} from "@remnacore/shared";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { ShopBotForm } from "@/components/shop-bot/ShopBotForm";
import { PageHeader, Panel, PanelHeader } from "@/components/ui";

export function ResellerBotPage() {
  const { t } = useTranslation();
  const { activeShopId, setActiveShop } = useShopStore();
  const { data: shops, isLoading } = useMyShops();
  const { data: botConfig } = useResellerBot(activeShopId);
  const { data: botPlugins } = useResellerBotPlugins(activeShopId);
  const updateBot = useUpdateResellerBot();

  // Auto-select when exactly 1 shop and none is currently selected.
  useEffect(() => {
    const first = shops?.[0];
    if (shops?.length === 1 && !activeShopId && first) {
      setActiveShop(first.id);
    }
  }, [shops, activeShopId, setActiveShop]);

  if (isLoading) return <LoadingSpinner />;

  if (!shops || shops.length === 0) {
    return (
      <Panel>
        <PanelHeader title={t("reseller.bot.title")} />
        <div className="px-4 py-8 text-center text-[11px] uppercase tracking-[1px] text-t6">
          {t("reseller.bot.noShops")}
        </div>
      </Panel>
    );
  }

  const selectClassName =
    "w-full border bg-input px-3 py-3 text-[13px] tracking-[.5px] text-t1 outline-none focus:border-accent/45";

  return (
    <>
      <PageHeader title={t("reseller.bot.title")} />

      <Panel>
        <PanelHeader title={t("reseller.bot.shopPicker")} />
        <div className="p-4">
          <select
            value={activeShopId ?? ""}
            onChange={(e) => setActiveShop(e.target.value || null)}
            className={selectClassName}
            style={{ borderColor: "var(--line-strong)" }}
          >
            <option value="">{t("reseller.bot.selectShop")}</option>
            {shops.map((shop) => (
              <option key={shop.id} value={shop.id}>
                {shop.name}
              </option>
            ))}
          </select>
        </div>
      </Panel>

      {activeShopId && (
        <Panel>
          <PanelHeader title={t("admin.tenantBot.title")} />
          <ShopBotForm
            config={botConfig}
            plugins={botPlugins}
            onSubmit={(data) => updateBot.mutate(data)}
            isPending={updateBot.isPending}
            isSuccess={updateBot.isSuccess}
          />
        </Panel>
      )}
    </>
  );
}
