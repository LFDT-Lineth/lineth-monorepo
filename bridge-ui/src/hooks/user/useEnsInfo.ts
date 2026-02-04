import { linea, mainnet } from "viem/chains";
import { useConnection, useEnsAvatar, useEnsName } from "wagmi";

export function useEnsInfo() {
  const { address } = useConnection();

  const { data: ensNameLinea } = useEnsName({
    address,
    chainId: linea.id,
    query: { enabled: Boolean(address), staleTime: Infinity },
  });

  const { data: ensNameMainnet } = useEnsName({
    address,
    chainId: mainnet.id,
    query: { enabled: Boolean(address) && !ensNameLinea, staleTime: Infinity },
  });

  const ensName = ensNameLinea ?? ensNameMainnet;
  const avatarChainId = ensNameLinea ? linea.id : ensNameMainnet ? mainnet.id : undefined;

  const { data: ensAvatar } = useEnsAvatar({
    name: ensName ?? undefined,
    chainId: avatarChainId,
    query: {
      enabled: Boolean(ensName),
      staleTime: Infinity,
    },
  });

  return {
    ensName,
    ensAvatar,
  };
}
