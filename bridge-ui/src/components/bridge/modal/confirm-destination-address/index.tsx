import Link from "next/link";
import { Address } from "viem";

import UnionIcon from "@/assets/icons/union.svg";
import Modal from "@/components/modal";
import Button from "@/components/ui/button";
import { buildExplorerUrl } from "@/lib/urls";
import { useChainStore } from "@/stores/chainStore";
import { formatAddress } from "@/utils/format";

import styles from "./confirm-destination-address.module.scss";

type Props = {
  isModalOpen: boolean;
  recipient: Address;
  onCloseModal: () => void;
  onConfirm: () => void;
};

export default function ConfirmDestinationAddress({ isModalOpen, recipient, onCloseModal, onConfirm }: Props) {
  const toChain = useChainStore.useToChain();
  const destinationAddressUrl = buildExplorerUrl(toChain.blockExplorers?.default.url, "address", recipient);

  return (
    <Modal title="Confirm destination address" isOpen={isModalOpen} onClose={onCloseModal}>
      <div className={styles["modal-inner"]}>
        <p className={styles["text"]}>
          Your funds are being bridged to the following address on the destination chain. Please review and confirm
          before proceeding.
        </p>
        {destinationAddressUrl ? (
          <Link href={destinationAddressUrl} target="_blank" rel="noopener noreferrer">
            {formatAddress(recipient)}
            <UnionIcon />
          </Link>
        ) : (
          <span>{formatAddress(recipient)}</span>
        )}
        <Button fullWidth onClick={onConfirm} data-testid="confirm-and-bridge-btn">
          Confirm and bridge
        </Button>
      </div>
    </Modal>
  );
}
