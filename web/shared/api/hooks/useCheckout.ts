import { useMutation } from "@tanstack/react-query";
import { ENDPOINTS } from "../endpoints.js";
import { apiPost } from "../client.js";
import type { CheckoutResult } from "../../types/index.js";
import type { StartCheckoutRequest } from "../types.js";

export function useStartCheckout() {
  return useMutation({
    mutationFn: (data: StartCheckoutRequest) =>
      apiPost<CheckoutResult>(ENDPOINTS.checkout.start, data),
  });
}
