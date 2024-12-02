import { useFormContext } from "react-hook-form";
import { parseUnits } from "viem";
import { useAccount, useBalance } from "wagmi";
import { MdInfo } from "react-icons/md";
import { NetworkLayer } from "@/config";
import { useBridge } from "@/hooks";
import { useChainStore } from "@/stores/chainStore";
import ApproveERC20 from "./ApproveERC20";
import { Button, Tooltip } from "../../ui";
import { cn } from "@/utils/cn";

type SubmitProps = {
  isLoading: boolean;
  isWaitingLoading: boolean;
};

export function Submit({ isLoading = false, isWaitingLoading = false }: SubmitProps) {
  // Form
  const { watch, formState } = useFormContext();
  const { errors } = formState;

  const [watchAmount, watchAllowance, watchClaim, watchBalance] = watch(["amount", "allowance", "claim", "balance"]);

  // Context
  const token = useChainStore((state) => state.token);
  const networkLayer = useChainStore((state) => state.networkLayer);
  const toChainId = useChainStore((state) => state.toChain?.id);

  // Wagmi
  const { bridgeEnabled } = useBridge();
  const { address } = useAccount();
  const { data: destinationChainBalance } = useBalance({
    address,
    chainId: toChainId,
    query: {
      enabled: !!address && !!toChainId,
    },
  });

  const destinationBalanceTooLow =
    watchClaim === "manual" && destinationChainBalance && destinationChainBalance.value === 0n;

  const originChainBalanceTooLow =
    token !== null &&
    (errors?.amount?.message !== undefined ||
      parseUnits(watchBalance, token.decimals) < parseUnits(watchAmount, token.decimals));

  const isERC20Token = token && networkLayer !== NetworkLayer.UNKNOWN && token[networkLayer];
  const isButtonDisabled = !bridgeEnabled(watchAmount, watchAllowance || BigInt(0), errors) || originChainBalanceTooLow;
  const isETHTransfer = token && token.symbol === "ETH";
  const showApproveERC20 =
    !isETHTransfer &&
    (!watchAllowance || (token?.decimals && watchAllowance < parseUnits(watchAmount, token.decimals)));

  const buttonText = originChainBalanceTooLow
    ? "Insufficient balance"
    : destinationBalanceTooLow
      ? "Bridge anyway"
      : "Bridge";

  return isETHTransfer ? (
    <Button
      type="submit"
      variant="primary"
      className={cn("w-full text-lg font-normal", {
        "bg-yellow border-none hover:bg-yellow": destinationBalanceTooLow,
      })}
      disabled={isButtonDisabled}
      loading={isLoading || isWaitingLoading}
    >
      {buttonText}
      {destinationBalanceTooLow && (
        <Tooltip
          text="You have selected Manual Claim and do not have ETH on the recipient chain to pay for gas. Click this to Bridge Anyway"
          className="z-[99] normal-case"
          position="bottom"
        >
          <MdInfo className="text-icon" />
        </Tooltip>
      )}
    </Button>
  ) : showApproveERC20 && isERC20Token ? (
    <ApproveERC20 />
  ) : (
    <Button
      id="submit-erc-btn"
      className={cn("w-full text-lg font-normal", {
        "bg-yellow border-none hover:bg-yellow": destinationBalanceTooLow,
      })}
      disabled={isButtonDisabled}
      loading={isLoading || isWaitingLoading}
      type="submit"
    >
      {buttonText}
      {destinationBalanceTooLow && (
        <Tooltip
          text="You have selected Manual Claim and do not have ETH on the recipient chain to pay for gas. Click this to Bridge Anyway"
          className="z-[100] normal-case"
          position="top"
        >
          <MdInfo className="text-icon" />
        </Tooltip>
      )}
    </Button>
  );
}
