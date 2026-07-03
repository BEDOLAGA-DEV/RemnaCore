import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { apiDeleteVoid, apiGet, apiPost } from "../client.js";
import { ENDPOINTS } from "../endpoints.js";

/** A tenant-defined custom role with its permission set. */
export type CustomRole = {
  id: string;
  name: string;
  description: string;
  scope_kind: "global" | "shop";
  tenant_id: string | null;
  permissions: string[];
};

/** A permission catalog entry offered when building a custom role. */
export type PermissionDef = {
  key: string;
  description: string;
  scope: "platform" | "shop";
};

export type CreateCustomRoleRequest = {
  name: string;
  description: string;
  scope_kind: "global" | "shop";
  tenant_id?: string | null;
  permissions: string[];
};

export function useCustomRoles(tenantId?: string) {
  return useQuery({
    queryKey: QUERY_KEYS.admin.roles.all(tenantId),
    queryFn: () => {
      const url = tenantId
        ? `${ENDPOINTS.roles.list}?tenant_id=${encodeURIComponent(tenantId)}`
        : ENDPOINTS.roles.list;
      return apiGet<{ roles: CustomRole[] }>(url).then((r) => r.roles ?? []);
    },
  });
}

export function usePermissionCatalog() {
  return useQuery({
    queryKey: QUERY_KEYS.admin.permissions,
    queryFn: () =>
      apiGet<{ permissions: PermissionDef[] }>(
        ENDPOINTS.roles.permissions,
      ).then((r) => r.permissions ?? []),
  });
}

export function useCreateCustomRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateCustomRoleRequest) =>
      apiPost<{ role_id: string }>(ENDPOINTS.roles.create, data),
    onSuccess: (_res, vars) => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.roles.all(vars.tenant_id ?? undefined),
      });
    },
  });
}

export function useDeleteCustomRole() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (roleId: string) =>
      apiDeleteVoid(ENDPOINTS.roles.delete(roleId)),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "roles"] });
    },
  });
}

export function useAssignCustomRole(userId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: { role_id: string; tenant_id?: string | null }) =>
      apiPost<void>(ENDPOINTS.users.roles.assignCustom(userId), data),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: QUERY_KEYS.admin.users.detail(userId),
      });
    },
  });
}
