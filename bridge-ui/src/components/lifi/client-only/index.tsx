import { type PropsWithChildren } from "react";
import { useHydrated } from "@/hooks";

interface ClientOnlyProps extends PropsWithChildren {
  fallback?: React.ReactNode;
}

export function ClientOnly({ children, fallback = null }: ClientOnlyProps) {
  const hydrated = useHydrated();
  return hydrated ? children : fallback;
}
