import { useContext, useEffect, useState } from "react";
import { useAccount } from "wagmi";
import { useFormContext } from "react-hook-form";
import { NetworkLayer, TokenType } from "@/config";
import { useChainStore } from "@/stores/chainStore";
import ClaimingTypeOption from "./ClaimingTypeOption";
import { ModalContext } from "@/contexts/modal.context";
import ManualClaimModal from "../modals/ManualClaimModal";

export function ClaimingType() {
  const { handleShow, handleClose } = useContext(ModalContext);
  const token = useChainStore((state) => state.token);
  const networkLayer = useChainStore((state) => state.networkLayer);

  const { isConnected } = useAccount();
  const { setValue, register, watch } = useFormContext();
  const [isManualConfirmed, setIsManualConfirmed] = useState(false);

  const selectedClaimType = watch("claim");

  const isAutoDisabled = networkLayer === NetworkLayer.L2 || token?.type !== TokenType.ETH || !isConnected;

  useEffect(() => {
    if (networkLayer === NetworkLayer.L2 || token?.type !== TokenType.ETH) {
      setValue("claim", "manual");
    } else if (token?.type === TokenType.ETH && networkLayer === NetworkLayer.L1) {
      setValue("claim", "auto");
    } else if (isManualConfirmed) {
      setValue("claim", "manual");
    }
  }, [token, setValue, networkLayer, isManualConfirmed]);

  const handleManualClaimClick = () => {
    if (selectedClaimType !== "manual") {
      handleShow(
        <ManualClaimModal
          handleYesClose={() => {
            setIsManualConfirmed(true);
            setValue("claim", "manual");
            handleClose();
          }}
          handleNoClose={() => {
            setIsManualConfirmed(false);
            handleClose();
          }}
        />,
      );
    }
  };

  useEffect(() => {
    if (isManualConfirmed) {
      setValue("claim", "manual");
    }
  }, [isManualConfirmed, setValue]);

  return (
    <div className="form-control grid grid-flow-row gap-2 rounded-lg bg-backgroundColor p-2 sm:grid-flow-col sm:rounded-full">
      <ClaimingTypeOption
        id="claim-auto"
        value="auto"
        label="Automatic"
        tooltip="Your transaction will be automatically deposited to the receiving address on destination chain. Suitable for first time bridges"
        disabled={isAutoDisabled}
        isConnected={isConnected}
        register={register("claim")}
        isSelected={selectedClaimType === "auto"}
      />
      <ClaimingTypeOption
        id="claim-manual"
        value="manual"
        label="Manual Claim (Advanced)"
        tooltip="You will need to claim your transaction on the destination chain with an additional transaction that requires ETH on the destination chain"
        disabled={!isConnected}
        isConnected={isConnected}
        onClick={handleManualClaimClick}
        isSelected={selectedClaimType === "manual"}
      />
    </div>
  );
}
