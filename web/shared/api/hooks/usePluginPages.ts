import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import type {
  AggregatedPluginPage,
  PluginDocument,
} from "../../types/index.js";
import { apiDeleteVoid, apiGet, apiPost, apiPutVoid } from "../client.js";
import { ENDPOINTS } from "../endpoints.js";

const PLUGIN_PAGES_STALE_TIME_MS = 60_000;

/**
 * Fetch the list of pages declared by all enabled plugins.
 * The backend aggregates page definitions from each plugin's manifest.
 */
export function usePluginPages() {
  return useQuery({
    queryKey: QUERY_KEYS.admin.pluginPages,
    queryFn: () => apiGet<AggregatedPluginPage[]>(ENDPOINTS.admin.pluginPages),
    staleTime: PLUGIN_PAGES_STALE_TIME_MS,
  });
}

/**
 * Generic RPC call to a plugin function.
 * Triggers `POST /api/plugins/{slug}/rpc/{functionName}`.
 */
export function usePluginRpc<T = unknown>(
  pluginSlug: string,
  functionName: string,
) {
  return useMutation({
    mutationFn: (body?: Record<string, unknown>) =>
      apiPost<T>(ENDPOINTS.plugins.rpc(pluginSlug, functionName), body ?? {}),
  });
}

/**
 * Full CRUD interface for a plugin's named collection.
 * Returns list query + create/update/remove mutations with automatic cache invalidation.
 */
/**
 * Normalize a flat API response into PluginDocument shape.
 * Custom CRUD endpoints (e.g. /api/tariffs) return flat objects with `id`,
 * while the generic collection API wraps data in { id, plugin_slug, collection, data }.
 */
function wrapAsDocument(
  item: Record<string, unknown>,
  pluginSlug: string,
  collection: string,
): PluginDocument {
  const { id, created_at, updated_at, ...data } = item;
  return {
    id: String(id ?? ""),
    plugin_slug: pluginSlug,
    collection,
    data,
    created_at: String(created_at ?? ""),
    updated_at: String(updated_at ?? ""),
  };
}

export function usePluginCollection(
  pluginSlug: string,
  collection: string,
  crudUrl?: string,
) {
  const queryClient = useQueryClient();
  const queryKey = QUERY_KEYS.plugins.collections(pluginSlug, collection);

  // When a custom CRUD URL is provided, use it directly instead of generic collection endpoints.
  const listUrl =
    crudUrl || ENDPOINTS.plugins.collections.list(pluginSlug, collection);
  const itemUrl = (id: string) =>
    crudUrl
      ? `${crudUrl}/${id}`
      : ENDPOINTS.plugins.collections.item(pluginSlug, collection, id);

  const list = useQuery({
    queryKey,
    queryFn: async () => {
      if (crudUrl) {
        const raw = await apiGet<Record<string, unknown>[]>(listUrl);
        return raw.map((item) => wrapAsDocument(item, pluginSlug, collection));
      }
      return apiGet<PluginDocument[]>(listUrl);
    },
  });

  const create = useMutation({
    mutationFn: async (data: Record<string, unknown>) => {
      if (crudUrl) {
        const raw = await apiPost<Record<string, unknown>>(listUrl, data);
        return wrapAsDocument(raw, pluginSlug, collection);
      }
      return apiPost<PluginDocument>(listUrl, data);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });

  const update = useMutation({
    mutationFn: ({ id, data }: { id: string; data: Record<string, unknown> }) =>
      apiPutVoid(itemUrl(id), data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });

  const remove = useMutation({
    mutationFn: (id: string) => apiDeleteVoid(itemUrl(id)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey });
    },
  });

  return { list, create, update, remove, queryKey };
}
