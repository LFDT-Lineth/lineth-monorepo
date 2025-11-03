"use client";

import { useAccount } from "wagmi";
import Button from "@/components/ui/button";
import styles from "./header-connect.module.scss";
import { useWeb3AuthConnect, useWeb3AuthDisconnect } from "@web3auth/modal/react";

export default function Connect() {
  const { address, isConnected } = useAccount();
  const { connect } = useWeb3AuthConnect();
  const { disconnect } = useWeb3AuthDisconnect();

  if (isConnected && address) {
    return (
      <div className={styles.connected}>
        <Button className={styles.disconnectButton} onClick={() => disconnect()}>
          Disconnect
        </Button>
      </div>
    );
  }

  return (
    <Button className={styles.connectButton} onClick={connect}>
      Connect
    </Button>
  );
}
