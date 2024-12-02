import Image from "next/image";
import { MdKeyboardArrowDown } from "react-icons/md";
import { ModalContext } from "@/contexts/modal.context";
import { useChainStore } from "@/stores/chainStore";
import { useContext } from "react";
import { useAccount } from "wagmi";
import TokenModal from "./modals/TokenModal";
import { useFormContext } from "react-hook-form";
import { Button } from "../ui";

export default function TokenList() {
  const token = useChainStore((state) => state.token);
  const { isConnected } = useAccount();

  const { handleShow } = useContext(ModalContext);
  const { setValue, clearErrors } = useFormContext();

  return (
    <div className="dropdown">
      {token && (
        <Button
          id="token-select-btn"
          type="button"
          variant="outline"
          className="border-none bg-cardBg px-2 py-1 font-normal hover:bg-cardBg hover:text-card"
          disabled={!isConnected}
          onClick={() =>
            handleShow(<TokenModal setValue={setValue} clearErrors={clearErrors} />, {
              showCloseButton: false,
            })
          }
        >
          <Image src={token.image} alt={token.name} width={25} height={25} className="rounded-full" />
          {token.symbol}
          <MdKeyboardArrowDown size={20} />
        </Button>
      )}
    </div>
  );
}
