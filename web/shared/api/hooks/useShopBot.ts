import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { apiGet, apiPutVoid } from "../client.js";
import { ENDPOINTS } from "../endpoints.js";

export interface ShopBotConfig {
  enabled: boolean;
  cabinet_url: string;
  bot_username: string;
  has_token: boolean;
  bot_plugin: string;
}

export interface UpdateShopBotInput {
  bot_token: string;
  cabinet_url: string;
  enabled: boolean;
  bot_plugin: string;
}

export interface BotPluginInfo {
  slug: string;
  name: string;
  kind: "builtin" | "wasm";
  description?: string;
}

export function useTenantBot(tenantId: string) {
  return useQuery({
    queryKey: QUERY_KEYS.admin.tenants.bot(tenantId),
    queryFn: () => apiGet<ShopBotConfig>(ENDPOINTS.admin.tenants.bot(tenantId)),
    enabled: !!tenantId,
  });
}

export function useUpdateTenantBot() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      tenantId,
      data,
    }: {
      tenantId: string;
      data: UpdateShopBotInput;
    }) => apiPutVoid(ENDPOINTS.admin.tenants.bot(tenantId), data),
    onSuccess: (_data, { tenantId }) => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.tenants.bot(tenantId),
      });
    },
  });
}

export function useBotPlugins() {
  return useQuery({
    queryKey: QUERY_KEYS.admin.botPlugins,
    queryFn: () => apiGet<BotPluginInfo[]>(ENDPOINTS.admin.botPlugins),
  });
}
