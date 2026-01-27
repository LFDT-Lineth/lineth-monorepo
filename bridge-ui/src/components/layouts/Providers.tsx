import { type ReactNode } from "react";

import { ModalProvider } from "@/contexts/ModalProvider";
import { QueryProvider } from "@/contexts/query.context";
import { Web3Provider } from "@/contexts/Web3Provider";
import { getTokenConfig } from "@/services/tokenService";
import { TokenStoreProvider } from "@/stores";

type ProvidersProps = {
  children: ReactNode;
};

async function getTokenStoreInitialState() {
  const tokensList = await getTokenConfig();

  return { tokensList, selectedToken: tokensList.MAINNET[0] };
}

export async function Providers({ children }: ProvidersProps) {
  const tokensStoreInitialState = await getTokenStoreInitialState();

  return (
    <ModalProvider>
      <QueryProvider>
        <Web3Provider>
          <TokenStoreProvider initialState={tokensStoreInitialState}>{children}</TokenStoreProvider>
        </Web3Provider>
      </QueryProvider>
    </ModalProvider>
  );
}
