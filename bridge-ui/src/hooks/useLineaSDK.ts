import { useMemo } from "react";
import { LineaSDK, Network } from "@consensys/linea-sdk";
import { linea, lineaSepolia, mainnet, sepolia } from "viem/chains";
import { L1MessageServiceContract, L2MessageServiceContract } from "@consensys/linea-sdk/dist/lib/contracts";
import { useChainStore } from "@/stores";
import { CHAINS_RPC_URLS } from "@/constants";

export interface LineaSDKContracts {
  L1: L1MessageServiceContract;
  L2: L2MessageServiceContract;
}

const useLineaSDK = () => {
  const fromChain = useChainStore.useFromChain();

  const { lineaSDK, lineaSDKContracts } = useMemo(() => {
    let l1RpcUrl;
    let l2RpcUrl;
    if (fromChain.testnet) {
      l1RpcUrl = CHAINS_RPC_URLS[sepolia.id];
      l2RpcUrl = CHAINS_RPC_URLS[lineaSepolia.id];
    } else {
      l1RpcUrl = CHAINS_RPC_URLS[mainnet.id];
      l2RpcUrl = CHAINS_RPC_URLS[linea.id];
    }

    const sdk = new LineaSDK({
      l1RpcUrl,
      l2RpcUrl,
      network: `linea-${fromChain.testnet ? "sepolia" : "mainnet"}` as Network,
      mode: "read-only",
    });

    const newLineaSDKContracts: LineaSDKContracts = {
      L1: sdk.getL1Contract(),
      L2: sdk.getL2Contract(),
    };

    return { lineaSDK: sdk, lineaSDKContracts: newLineaSDKContracts };
  }, [fromChain.testnet]);

  return { lineaSDK, lineaSDKContracts };
};

export default useLineaSDK;
