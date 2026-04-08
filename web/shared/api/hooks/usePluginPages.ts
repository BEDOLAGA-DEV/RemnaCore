import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import type {
	AggregatedPluginPage,
	PluginDocument,
} from "../../types/index.js";
import { apiDelete, apiGet, apiPost, apiPut } from "../client.js";
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
export function usePluginCollection(pluginSlug: string, collection: string) {
	const queryClient = useQueryClient();
	const queryKey = QUERY_KEYS.plugins.collections(pluginSlug, collection);

	const list = useQuery({
		queryKey,
		queryFn: () =>
			apiGet<PluginDocument[]>(
				ENDPOINTS.plugins.collections.list(pluginSlug, collection),
			),
	});

	const create = useMutation({
		mutationFn: (data: Record<string, unknown>) =>
			apiPost<PluginDocument>(
				ENDPOINTS.plugins.collections.list(pluginSlug, collection),
				data,
			),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey });
		},
	});

	const update = useMutation({
		mutationFn: ({ id, data }: { id: string; data: Record<string, unknown> }) =>
			apiPut<PluginDocument>(
				ENDPOINTS.plugins.collections.item(pluginSlug, collection, id),
				data,
			),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey });
		},
	});

	const remove = useMutation({
		mutationFn: (id: string) =>
			apiDelete(ENDPOINTS.plugins.collections.item(pluginSlug, collection, id)),
		onSuccess: () => {
			queryClient.invalidateQueries({ queryKey });
		},
	});

	return { list, create, update, remove, queryKey };
}
