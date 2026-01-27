"use client";
import { useEffect } from "react";

import { useWeb3Auth, useWeb3AuthConnect } from "@web3auth/modal/react";
import { useAccount } from "wagmi";

import Button from "@/components/ui/button";
import UserAvatar from "@/components/user-avatar";
import { useModal } from "@/contexts/ModalProvider";
import { usePrefetchPoh } from "@/hooks/useCheckPoh";
import { useEnsInfo } from "@/hooks/user/useEnsInfo";

export default function HeaderConnect() {
  const { address } = useAccount();
  const { connect, loading: isConnecting, isConnected } = useWeb3AuthConnect();
  const { isInitializing } = useWeb3Auth();
  const { ensAvatar } = useEnsInfo();
  const { updateModal } = useModal();
  const prefetchPoh = usePrefetchPoh();

  const handleConnectionToggle = async () => {
    if (isConnected) {
      updateModal(true, "user-wallet");
    } else {
      await connect();
    }
  };

  useEffect(() => {
    if (address) {
      prefetchPoh(address);
    }
  }, [address, prefetchPoh]);

  if (isConnected) return <UserAvatar src={ensAvatar ?? ""} size="small" onClick={handleConnectionToggle} />;

  return (
    <Button variant="primary" fullWidth disabled={isConnecting || isInitializing} onClick={handleConnectionToggle}>
      Connect
    </Button>
  );
}
