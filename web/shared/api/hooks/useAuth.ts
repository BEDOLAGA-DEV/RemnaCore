import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useAuthStore } from "../../stores/authStore.js";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { STALE_TIME_USER_MS } from "../../lib/constants.js";
import { ENDPOINTS } from "../endpoints.js";
import { apiGet, apiPost, apiPut, apiDelete } from "../client.js";
import type { User, LoginResponse, RegisterResponse } from "../../types/index.js";
import type {
  LoginRequest,
  RegisterRequest,
  ForgotPasswordRequest,
  ResetPasswordRequest,
  UpdateProfileRequest,
  LinkTelegramRequest,
  StatusResponse,
  VerifyEmailRequest,
} from "../types.js";

export function useMe() {
  const { isAuthenticated } = useAuthStore();
  return useQuery({
    queryKey: QUERY_KEYS.auth.me,
    queryFn: () => apiGet<User>(ENDPOINTS.me.get),
    enabled: isAuthenticated,
    staleTime: STALE_TIME_USER_MS,
  });
}

export function useLogin() {
  const { login } = useAuthStore();
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: LoginRequest) =>
      apiPost<LoginResponse>(ENDPOINTS.auth.login, data),
    onSuccess: (result) => {
      login(result.access_token, result.refresh_token, result.user);
      queryClient.setQueryData(QUERY_KEYS.auth.me, result.user);
    },
  });
}

export function useRegister() {
  return useMutation({
    mutationFn: (data: RegisterRequest) =>
      apiPost<RegisterResponse>(ENDPOINTS.auth.register, data),
  });
}

export function useVerifyEmail() {
  return useMutation({
    mutationFn: (data: VerifyEmailRequest) =>
      apiPost<StatusResponse>(ENDPOINTS.auth.verifyEmail, data),
  });
}

export function useForgotPassword() {
  return useMutation({
    mutationFn: (data: ForgotPasswordRequest) =>
      apiPost<StatusResponse>(ENDPOINTS.auth.forgotPassword, data),
  });
}

export function useResetPassword() {
  return useMutation({
    mutationFn: (data: ResetPasswordRequest) =>
      apiPost<StatusResponse>(ENDPOINTS.auth.resetPassword, data),
  });
}

export function useUpdateProfile() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: UpdateProfileRequest) =>
      apiPut<StatusResponse>(ENDPOINTS.me.update, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.auth.me });
    },
  });
}

export function useLinkTelegram() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: LinkTelegramRequest) =>
      apiPost<StatusResponse>(ENDPOINTS.me.linkTelegram, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.auth.me });
    },
  });
}

export function useUnlinkTelegram() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () =>
      apiDelete<StatusResponse>(ENDPOINTS.me.unlinkTelegram),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.auth.me });
    },
  });
}

export function useLogout() {
  const { logout } = useAuthStore();
  const queryClient = useQueryClient();

  return () => {
    logout();
    queryClient.clear();
  };
}
