"use client";

import { useAccount, useConnect, useConnectors, useDisconnect } from "wagmi";
import Button from "@/components/ui/button";
import styles from "./header-connect.module.scss";

export default function Connect() {
  const { address, isConnected } = useAccount();
  const { connectAsync } = useConnect();
  const { disconnect } = useDisconnect();
  const connectors = useConnectors();

  const handleConnect = async () => {
    const web3authConnector = connectors.find((c) => c.id === "web3auth");
    if (web3authConnector) {
      await connectAsync({ connector: web3authConnector });
    }
  };

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
    <Button className={styles.connectButton} onClick={handleConnect}>
      Connect
    </Button>
  );
}
