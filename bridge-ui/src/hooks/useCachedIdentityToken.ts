import { useIdentityToken } from "@web3auth/modal/react";
import { useCallback } from "react";

/**
 * Global cache for Web3Auth identity token (shared across all components)
 * This prevents multiple signature requests when multiple hooks request token simultaneously
 */
let globalTokenCache: { token: string; timestamp: number } | null = null;
let globalPendingRequest: Promise<string | null> | null = null;

const CACHE_DURATION = 10 * 24 * 60 * 60 * 1000; // 10 days

/**
 * Hook to get cached Web3Auth identity token
 *
 * Web3Auth generates a new token with signature on each `getIdentityToken()` call.
 * This hook uses a GLOBAL cache so all components share the same token,
 * avoiding multiple signature prompts.
 */
export function useCachedIdentityToken() {
  const { getIdentityToken: getToken } = useIdentityToken();

  const getIdentityToken = useCallback(async (): Promise<string | null> => {
    // Check if we have a valid cached token
    if (globalTokenCache) {
      const age = Date.now() - globalTokenCache.timestamp;
      if (age < CACHE_DURATION) {
        return globalTokenCache.token;
      }
    }

    // If a request is already in progress, wait for it instead of making a new one
    if (globalPendingRequest) {
      return globalPendingRequest;
    }

    // Start new request
    globalPendingRequest = (async () => {
      try {
        // Get fresh token from Web3Auth (triggers signature)
        const token = await getToken();

        // Cache it globally
        if (token) {
          globalTokenCache = {
            token,
            timestamp: Date.now(),
          };
        }

        return token;
      } finally {
        // Clear pending request when done
        globalPendingRequest = null;
      }
    })();

    return globalPendingRequest;
  }, [getToken]);

  const clearTokenCache = useCallback(() => {
    globalTokenCache = null;
    globalPendingRequest = null;
  }, []);

  // Synchronous check if a token is currently available (either cached or pending)
  const hasToken = useCallback((): boolean => {
    // Check if we have a valid cached token
    if (globalTokenCache) {
      const age = Date.now() - globalTokenCache.timestamp;
      if (age < CACHE_DURATION) {
        return true;
      }
    }
    // If a request is pending, we should wait for it (treat as "has token coming")
    return globalPendingRequest !== null;
  }, []);

  return {
    getIdentityToken,
    clearTokenCache,
    hasToken,
  };
}
