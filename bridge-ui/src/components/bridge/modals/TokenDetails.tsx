"use client";

import React, { useEffect } from "react";
import Image from "next/image";
import { FieldValues, UseFormClearErrors, UseFormSetValue } from "react-hook-form";
import { useBlockNumber } from "wagmi";
import { config } from "@/config";
import { formatBalance } from "@/utils/format";
import { NetworkLayer, NetworkType, TokenInfo, TokenType } from "@/config/config";
import { useChainStore } from "@/stores/chainStore";
import { cn } from "@/utils/cn";
import { useTokenBalance } from "@/hooks/useTokenBalance";

interface TokenDetailsProps {
  token: TokenInfo;
  onTokenClick: (token: TokenInfo) => void;
  setValue: UseFormSetValue<FieldValues>;
  clearErrors: UseFormClearErrors<FieldValues>;
  tokenPrice?: number;
}

export default function TokenDetails({ token, onTokenClick, setValue, clearErrors, tokenPrice }: TokenDetailsProps) {
  const { networkLayer, setToken, setTokenBridgeAddress, networkType } = useChainStore((state) => ({
    networkLayer: state.networkLayer,
    setToken: state.setToken,
    setTokenBridgeAddress: state.setTokenBridgeAddress,
    networkType: state.networkType,
  }));

  const tokenNotFromCurrentLayer = !token[networkLayer] && token.type !== TokenType.ETH;

  const { data: blockNumber } = useBlockNumber({ watch: true });
  const { balance, refetch } = useTokenBalance(token[networkLayer], token?.decimals);

  useEffect(() => {
    if (blockNumber && blockNumber % 5n === 0n) {
      refetch();
    }
  }, [blockNumber, refetch]);

  return (
    <button
      id={`token-details-${token.symbol}-btn`}
      className={cn(
        "flex items-center justify-between w-full px-4 py-3 bg-transparent border-0 hover:bg-primary-light",
        {
          "btn-disabled": tokenNotFromCurrentLayer,
        },
      )}
      disabled={tokenNotFromCurrentLayer}
      onClick={() => {
        if (networkLayer !== NetworkLayer.UNKNOWN && token && networkType !== NetworkType.WRONG_NETWORK) {
          setValue("amount", "");
          clearErrors("amount");
          setToken(token);
          switch (token.type) {
            case TokenType.USDC:
              setTokenBridgeAddress(config.networks[networkType][networkLayer].usdcBridgeAddress);
              break;
            default:
              setTokenBridgeAddress(config.networks[networkType][networkLayer].tokenBridgeAddress);
              break;
          }

          onTokenClick(token);
        }
      }}
    >
      <div className="flex gap-5">
        <Image src={token.image} alt={token.name} width={40} height={40} className="rounded-full" />
        <div className="text-left">
          <p className="font-semibold">{token.symbol}</p>
          <p className="text-sm font-normal text-[#898989]">{token.name}</p>
        </div>
      </div>
      {!tokenNotFromCurrentLayer && (
        <div className="text-right">
          <p className="font-semibold">
            {formatBalance(balance)} {token.symbol}
          </p>
          {tokenPrice ? (
            <p className="text-[#898989]">
              {(tokenPrice * Number(balance)).toLocaleString("en-US", {
                style: "currency",
                currency: "USD",
                maximumFractionDigits: 4,
              })}
            </p>
          ) : null}
        </div>
      )}
      {tokenNotFromCurrentLayer && (
        <div className="ml-10 text-left text-warning">
          <p>Token is from other layer. Please swap networks to import token.</p>
        </div>
      )}
    </button>
  );
}
