import { useMemo } from "react";
import { encodeFunctionData, padHex, zeroHash } from "viem";
import { useFormStore, useChainStore } from "@/stores";
import { isCctp } from "@/utils/tokens";
import { useCctpFee, useCctpDestinationDomain } from "./useCctpUtilHooks";
import { CCTP_MAX_FINALITY_THRESHOLD, CCTP_MIN_FINALITY_THRESHOLD } from "@/constants";
import { isNull, isUndefined, isUndefinedOrEmptyString } from "@/utils";
import { CCTPMode } from "@/types";

type UseDepositForBurnTxArgs = {
  allowance?: bigint;
};

const useDepositForBurnTxArgs = ({ allowance }: UseDepositForBurnTxArgs) => {
  const fromChain = useChainStore.useFromChain();
  const cctpDestinationDomain = useCctpDestinationDomain();
  const token = useFormStore((state) => state.token);
  const amount = useFormStore((state) => state.amount);
  const recipient = useFormStore((state) => state.recipient);
  const cctpMode = useFormStore((state) => state.cctpMode);
  const fee = useCctpFee(amount, token.decimals);

  return useMemo(() => {
    if (
      isNull(amount) ||
      isNull(fee) ||
      isUndefined(allowance) ||
      allowance < amount ||
      isUndefinedOrEmptyString(recipient) ||
      !isCctp(token)
    ) {
      return;
    }

    return {
      type: "depositForBurn",
      args: {
        to: fromChain.cctpTokenMessengerV2Address,
        data: encodeFunctionData({
          abi: [
            {
              type: "function",
              name: "depositForBurn",
              stateMutability: "nonpayable",
              inputs: [
                { name: "amount", type: "uint256" },
                { name: "destinationDomain", type: "uint32" },
                { name: "mintRecipient", type: "bytes32" },
                { name: "burnToken", type: "address" },
                { name: "destinationCaller", type: "bytes32" },
                { name: "maxFee", type: "uint256" },
                { name: "minFinalityThreshold", type: "uint32" },
              ],
              outputs: [],
            },
          ],
          functionName: "depositForBurn",
          args: [
            amount,
            cctpDestinationDomain,
            padHex(recipient),
            token[fromChain.layer],
            zeroHash,
            fee,
            cctpMode === CCTPMode.FAST ? CCTP_MAX_FINALITY_THRESHOLD : CCTP_MIN_FINALITY_THRESHOLD,
          ],
        }),
        value: 0n,
        chainId: fromChain.id,
      },
    };
  }, [allowance, amount, fee, cctpDestinationDomain, fromChain, recipient, token, cctpMode]);
};

export default useDepositForBurnTxArgs;
