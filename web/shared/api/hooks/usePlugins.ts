import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";
import { apiGet, apiPost, apiPut, apiDelete } from "../client.js";
import type { Plugin } from "../../types/index.js";
import type {
  InstallPluginRequest,
  UpdatePluginConfigRequest,
  StatusResponse,
} from "../types.js";

export function usePlugins() {
  return useQuery({
    queryKey: QUERY_KEYS.admin.plugins.all,
    queryFn: () => apiGet<Plugin[]>(ENDPOINTS.admin.plugins.list),
  });
}

export function usePlugin(pluginId: string) {
  return useQuery({
    queryKey: QUERY_KEYS.admin.plugins.detail(pluginId),
    queryFn: () => apiGet<Plugin>(ENDPOINTS.admin.plugins.detail(pluginId)),
    enabled: !!pluginId,
  });
}

export function useInstallPlugin() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: InstallPluginRequest) =>
      apiPost<Plugin>(ENDPOINTS.admin.plugins.install, data),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.plugins.all,
      });
    },
  });
}

export function useEnablePlugin() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (pluginId: string) =>
      apiPost<StatusResponse>(ENDPOINTS.admin.plugins.enable(pluginId)),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.plugins.all,
      });
    },
  });
}

export function useDisablePlugin() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (pluginId: string) =>
      apiPost<StatusResponse>(ENDPOINTS.admin.plugins.disable(pluginId)),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.plugins.all,
      });
    },
  });
}

export function useUninstallPlugin() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (pluginId: string) =>
      apiDelete<StatusResponse>(ENDPOINTS.admin.plugins.uninstall(pluginId)),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.plugins.all,
      });
    },
  });
}

export function useUpdatePluginConfig() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      pluginId,
      data,
    }: {
      pluginId: string;
      data: UpdatePluginConfigRequest;
    }) =>
      apiPut<StatusResponse>(
        ENDPOINTS.admin.plugins.updateConfig(pluginId),
        data,
      ),
    onSuccess: (_data, { pluginId }) => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.plugins.detail(pluginId),
      });
    },
  });
}
