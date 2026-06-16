import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "../../stores/authStore.js";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";
import { apiGet, apiPost } from "../client.js";
import type { LoginResponse, SetupStatusResponse } from "../../types/index.js";
import type { CreateAdminRequest } from "../types.js";

/**
 * Queries whether the platform still needs first-run admin setup. The result is
 * stable for the session (it only flips once, when the first admin is created).
 */
export function useSetupStatus() {
  return useQuery({
    queryKey: ["setup", "status"],
    queryFn: () => apiGet<SetupStatusResponse>(ENDPOINTS.setup.status),
    staleTime: Number.POSITIVE_INFINITY,
  });
}

/**
 * Creates the first administrator and logs them in (mirrors useLogin's success
 * handling). The backend returns 409 once an admin already exists.
 */
export function useSetupAdmin() {
  const { login } = useAuthStore();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateAdminRequest) =>
      apiPost<LoginResponse>(ENDPOINTS.setup.admin, data),
    onSuccess: (result) => {
      login(result.access_token, result.refresh_token, result.user);
      queryClient.setQueryData(QUERY_KEYS.auth.me, result.user);
    },
  });
}
