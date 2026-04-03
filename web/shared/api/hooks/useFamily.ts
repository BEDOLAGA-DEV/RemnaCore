import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { ENDPOINTS } from "../endpoints.js";
import { apiGet, apiPost, apiDelete } from "../client.js";
import type { FamilyGroup } from "../../types/index.js";
import type {
  CreateFamilyRequest,
  AddFamilyMemberRequest,
  StatusResponse,
} from "../types.js";

export function useFamily() {
  return useQuery({
    queryKey: QUERY_KEYS.family.mine,
    queryFn: () => apiGet<FamilyGroup>(ENDPOINTS.family.get),
    retry: false,
  });
}

export function useCreateFamily() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateFamilyRequest) =>
      apiPost<FamilyGroup>(ENDPOINTS.family.create, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.family.mine });
    },
  });
}

export function useAddFamilyMember() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: AddFamilyMemberRequest) =>
      apiPost<StatusResponse>(ENDPOINTS.family.addMember, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.family.mine });
    },
  });
}

export function useRemoveFamilyMember() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      userId,
      subscriptionId,
    }: {
      userId: string;
      subscriptionId: string;
    }) =>
      apiDelete<StatusResponse>(ENDPOINTS.family.removeMember(userId), {
        subscription_id: subscriptionId,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUERY_KEYS.family.mine });
    },
  });
}
