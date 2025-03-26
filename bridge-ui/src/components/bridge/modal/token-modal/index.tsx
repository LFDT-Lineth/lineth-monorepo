"use client";

import { useCallback, useMemo, useState } from "react";
import { isAddress, getAddress, Address, zeroAddress } from "viem";
import Modal from "@/components/modal";
import SearchIcon from "@/assets/icons/search.svg";
import styles from "./token-modal.module.scss";
import TokenDetails from "./token-details";
import { useDevice, useTokenPrices, useTokens } from "@/hooks";
import { useTokenStore, useChainStore, useConfigStore } from "@/stores";
import { Token } from "@/types";
import { safeGetAddress, isEmptyObject, isEth } from "@/utils";
import { useAccount } from "wagmi";

interface TokenModalProps {
  isModalOpen: boolean;
  onCloseModal: () => void;
}

export default function TokenModal({ isModalOpen, onCloseModal }: TokenModalProps) {
  const { isConnected } = useAccount();
  const tokensList = useTokens();
  const setSelectedToken = useTokenStore((state) => state.setSelectedToken);
  const fromChain = useChainStore.useFromChain();
  const currency = useConfigStore((state) => state.currency);
  const { isMobile } = useDevice();

  const [searchQuery, setSearchQuery] = useState("");

  const chainLayer = fromChain.layer;
  const chainId = fromChain.id;
  const isTestnet = fromChain.testnet;

  const filteredTokens = useMemo(() => {
    if (!searchQuery) return tokensList;
    const query = searchQuery.toLowerCase();

    return tokensList.filter((token: Token) => {
      const rawAddress = chainLayer ? token[chainLayer] : undefined;
      const tokenAddress = rawAddress ? safeGetAddress(rawAddress) : undefined;
      return (
        (tokenAddress || isEth(token)) &&
        (token.name.toLowerCase().includes(query) ||
          token.symbol.toLowerCase().includes(query) ||
          (tokenAddress && tokenAddress.toLowerCase().includes(query)))
      );
    });
  }, [tokensList, searchQuery, chainLayer]);

  const tokenAddresses = useMemo(
    () =>
      filteredTokens.map((token) => {
        if (token.name === "Ether") return zeroAddress;
        const rawAddress = chainLayer ? token[chainLayer] : null;
        const tokenAddress = rawAddress ? safeGetAddress(rawAddress) : null;
        return (tokenAddress ?? zeroAddress) as Address;
      }),
    [filteredTokens, chainLayer],
  );

  const { data: tokenPrices } = useTokenPrices(tokenAddresses, chainId);

  const handleTokenClick = useCallback(
    (token: Token) => {
      setSelectedToken(token);
      onCloseModal();
    },
    [onCloseModal, setSelectedToken],
  );

  const getTokenPrice = useCallback(
    (token: Token): number | undefined => {
      if (fromChain && !isTestnet && !isEmptyObject(tokenPrices)) {
        const rawAddress = token[chainLayer];
        const tokenAddress = (safeGetAddress(rawAddress) || zeroAddress).toLowerCase();
        return tokenPrices[tokenAddress];
      }
      return undefined;
    },
    [tokenPrices, fromChain, chainLayer, isTestnet],
  );

  const normalizeInput = useCallback((input: string): string => {
    return isAddress(input) ? getAddress(input) : input.toLowerCase();
  }, []);

  const handleSearchChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      setSearchQuery(normalizeInput(e.target.value));
    },
    [normalizeInput],
  );

  return (
    <Modal title="Select a token" isOpen={isModalOpen} onClose={onCloseModal} isDrawer={isMobile}>
      <div className={styles["modal-inner"]}>
        <div className={styles["input-wrapper"]}>
          <SearchIcon />
          <input
            type="text"
            placeholder="Search by token name, symbol or address"
            onChange={handleSearchChange}
            value={searchQuery}
          />
        </div>
        <div className={styles["list-token"]}>
          {filteredTokens.length > 0 ? (
            filteredTokens.map((token: Token, index: number) => (
              <TokenDetails
                isConnected={isConnected}
                token={token}
                onTokenClick={handleTokenClick}
                key={`token-details-${token.symbol}-${index}`}
                tokenPrice={getTokenPrice(token)}
                currency={currency}
              />
            ))
          ) : (
            <div className={styles["not-found"]}>
              <p>Sorry, there are no results for that term. Please enter a valid token name or address.</p>
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}
