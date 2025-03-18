"use client";

import { useMemo, useState } from "react";
import SearchIcon from "@/assets/icons/search.svg";
import Modal from "@/components/modal";
import { useDevice } from "@/hooks";
import NetworkDetails from "./network-details";
import styles from "./select-network.module.scss";
import { Chain } from "@/types";

interface Props {
  networks: Chain[];
  isModalOpen: boolean;
  onCloseModal: () => void;
  onClick: (chain: Chain) => void;
}

export default function SelectNetwork({ isModalOpen, onCloseModal, onClick, networks }: Props) {
  const { isMobile } = useDevice();
  const [searchQuery, setSearchQuery] = useState("");

  const filteredNetworks = useMemo(() => {
    if (!searchQuery) return networks;
    const query = searchQuery.toLowerCase();
    return networks.filter((network) => network.name.toLowerCase().startsWith(query));
  }, [networks, searchQuery]);

  return (
    <Modal title="Select a network" isOpen={isModalOpen} onClose={onCloseModal} isDrawer={isMobile}>
      <div className={styles["modal-inner"]}>
        <div className={styles["input-wrapper"]}>
          <SearchIcon />
          <input
            type="text"
            placeholder="Search by name"
            onChange={({ target: { value } }) => setSearchQuery(value)}
            value={searchQuery}
          />
        </div>
        <div className={styles["list-network"]}>
          {filteredNetworks && filteredNetworks.length > 0 ? (
            filteredNetworks.map((network, index: number) => {
              return (
                <NetworkDetails
                  key={index}
                  name={network.name}
                  onClickNetwork={() => {
                    onClick(network);
                    onCloseModal();
                  }}
                  image={network.iconPath}
                />
              );
            })
          ) : (
            <div className={styles["not-found"]}>
              <p>No networks found</p>
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}
