import { useSyncExternalStore } from "react";

function subscribe() {
  return () => {};
}

const useHydrated = () => {
  return useSyncExternalStore(
    subscribe,
    () => true,
    () => false,
  );
};

export default useHydrated;
