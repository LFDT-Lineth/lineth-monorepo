"use client";

import { useEffect, useRef } from "react";
import Image from "next/image";
import { useAccount, useDisconnect } from "wagmi";
import { useChainStore } from "@/stores/chainStore";
import DropdownItem from "@/components/DropdownItem";
import ConnectButton from "@/components/ConnectButton";
import { formatAddress } from "@/utils/format";

export function Wallet() {
  const { address, isConnected } = useAccount();
  const { disconnect } = useDisconnect();

  const fromChain = useChainStore((state) => state.fromChain);

  const detailsRef = useRef<HTMLDetailsElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (detailsRef.current && !detailsRef.current.contains(e.target as Node)) {
        detailsRef.current.removeAttribute("open");
      }
    };
    document.addEventListener("click", handleClickOutside);
    return () => {
      document.removeEventListener("click", handleClickOutside);
    };
  }, []);

  const handleCopy = async () => {
    if (!address) return;

    try {
      await navigator.clipboard.writeText(address);
    } catch (err) {
      console.error("Failed to copy: ", err);
    }
  };

  if (isConnected) {
    return (
      <details className="dropdown relative" ref={detailsRef}>
        <summary className="flex cursor-pointer items-center gap-2 rounded-full bg-cardBg p-2 px-3">
          <Image
            src={"/images/logo/metamask.svg"}
            alt="MetaMask"
            width={0}
            height={0}
            style={{ width: "18px", height: "auto" }}
          />

          <span className="hidden md:block">{formatAddress(address)}</span>
        </summary>
        <ul className="menu dropdown-content absolute right-0 z-10 mt-2 min-w-max bg-cardBg p-0 shadow">
          <DropdownItem title="Copy address" onClick={handleCopy} />
          <DropdownItem title="Explorer" externalLink={fromChain?.blockExplorers?.default.url} />
          <DropdownItem title="Logout" onClick={() => disconnect()} />
        </ul>
      </details>
    );
  }

  return (
    <div className="p-0">
      <ConnectButton />
    </div>
  );
}
