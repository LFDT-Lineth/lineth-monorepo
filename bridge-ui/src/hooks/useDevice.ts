import { useCallback, useSyncExternalStore } from "react";

const useMediaQuery = (query: string) => {
  const subscribe = useCallback(
    (callback: () => void) => {
      const mediaQuery = window.matchMedia(query);
      if (mediaQuery.media === "invalid" || mediaQuery.media === "not all") {
        throw new Error("useMediaQuery: The provided media query is invalid or not supported.");
      }
      mediaQuery.addEventListener("change", callback);
      return () => mediaQuery.removeEventListener("change", callback);
    },
    [query],
  );

  const getSnapshot = useCallback(() => {
    return window.matchMedia(query).matches;
  }, [query]);

  const getServerSnapshot = () => false;

  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
};

const useDevice = () => {
  const isMobile = useMediaQuery("(max-width: 767px)");
  const isTablet = useMediaQuery("(min-width: 768px) and (max-width: 1023px)");
  const isDesktop = useMediaQuery("(min-width: 1024px)");

  return {
    isMobile,
    isTablet,
    isDesktop,
  };
};

export default useDevice;
