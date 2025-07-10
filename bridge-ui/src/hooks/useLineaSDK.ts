import { useMemo } from "react";
import { LineaSDK, Network } from "@consensys/linea-sdk";
import { linea, lineaSepolia, mainnet, sepolia } from "viem/chains";
import { L1MessageServiceContract, L2MessageServiceContract } from "@consensys/linea-sdk/dist/lib/contracts";
import { useChainStore } from "@/stores";
import { CHAINS_RPC_URLS, localL1Network, localL2Network } from "@/constants";
import { config } from "@/config";

export interface LineaSDKContracts {
  L1: L1MessageServiceContract;
  L2: L2MessageServiceContract;
}

const useLineaSDK = () => {
  const fromChain = useChainStore.useFromChain();

  const { lineaSDK, lineaSDKContracts } = useMemo(() => {
    let l1RpcUrl;
    let l2RpcUrl;

    if (fromChain.testnet && !fromChain.localNetwork) {
      l1RpcUrl = CHAINS_RPC_URLS[sepolia.id];
      l2RpcUrl = CHAINS_RPC_URLS[lineaSepolia.id];
    } else if (fromChain.localNetwork) {
      l1RpcUrl = localL1Network.rpcUrls.default.http[0];
      l2RpcUrl = localL2Network.rpcUrls.default.http[0];
    } else {
      l1RpcUrl = CHAINS_RPC_URLS[mainnet.id];
      l2RpcUrl = CHAINS_RPC_URLS[linea.id];
    }

    const sdk = new LineaSDK({
      l1RpcUrl,
      l2RpcUrl,
      network: fromChain.localNetwork ? "localhost" : (`linea-${fromChain.testnet ? "sepolia" : "mainnet"}` as Network),
      mode: "read-only",
    });

    const newLineaSDKContracts: LineaSDKContracts = {
      L1: sdk.getL1Contract(
        config.e2eTestMode ? config.chains[localL1Network.id].messageServiceAddress : undefined,
        config.e2eTestMode ? config.chains[localL2Network.id].messageServiceAddress : undefined,
      ),
      L2: sdk.getL2Contract(config.e2eTestMode ? config.chains[localL2Network.id].messageServiceAddress : undefined),
    };

    return { lineaSDK: sdk, lineaSDKContracts: newLineaSDKContracts };
  }, [fromChain.testnet, fromChain.localNetwork]);

  return { lineaSDK, lineaSDKContracts };
};

export default useLineaSDK;
