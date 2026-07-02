import { useQuery } from "@tanstack/react-query";
import { QUERY_KEYS } from "../../lib/queryKeys.js";
import { apiGet } from "../client.js";
import { ENDPOINTS } from "../endpoints.js";

/** One shop (tenant) the authenticated user is a member of. */
export type MyShop = {
  id: string;
  name: string;
};

/** Lists the caller's shop memberships (GET /api/me/shops). */
export function useMyShops() {
  return useQuery({
    queryKey: QUERY_KEYS.auth.myShops,
    queryFn: () => apiGet<MyShop[]>(ENDPOINTS.me.shops),
  });
}
