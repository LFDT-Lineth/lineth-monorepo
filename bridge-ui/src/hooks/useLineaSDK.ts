import { useMemo } from "react";
import { LineaSDK, Network } from "@consensys/linea-sdk";
import { L1MessageServiceContract, L2MessageServiceContract } from "@consensys/linea-sdk/dist/lib/contracts";
import { useChainStore } from "@/stores";

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
      l1RpcUrl = `https://sepolia.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`;
      l2RpcUrl = `https://linea-sepolia.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`;
    } else {
      l1RpcUrl = `https://mainnet.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`;
      l2RpcUrl = `https://linea-mainnet.infura.io/v3/${process.env.NEXT_PUBLIC_INFURA_ID}`;
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
