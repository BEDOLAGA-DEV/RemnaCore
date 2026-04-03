import { useMutation, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";
import { apiPost, apiPut } from "../client.js";
import type { Tenant } from "../../types/index.js";
import type {
  CreateTenantRequest,
  CreateTenantResponse,
  UpdateBrandingRequest,
} from "../types.js";

export function useCreateTenant() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateTenantRequest) =>
      apiPost<CreateTenantResponse>(ENDPOINTS.admin.tenants.create, data),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.tenants.root,
      });
    },
  });
}

export function useUpdateBranding() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      tenantId,
      data,
    }: {
      tenantId: string;
      data: UpdateBrandingRequest;
    }) =>
      apiPut<Tenant>(
        ENDPOINTS.admin.tenants.updateBranding(tenantId),
        data,
      ),
    onSuccess: (_data, { tenantId }) => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.tenants.detail(tenantId),
      });
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.tenants.root,
      });
    },
  });
}
