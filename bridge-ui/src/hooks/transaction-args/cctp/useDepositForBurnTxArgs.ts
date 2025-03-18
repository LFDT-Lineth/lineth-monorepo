import { useMemo } from "react";
import { useAccount } from "wagmi";
import { encodeFunctionData, padHex, zeroHash } from "viem";
import { useFormStore, useChainStore } from "@/stores";
import { isCctp } from "@/utils/tokens";
import useCctpDestinationDomain from "./useCctpDestinationDomain";
import { CCTP_MIN_FINALITY_THRESHOLD, CCTP_TOKEN_MESSENGER, CCTP_TRANSFER_MAX_FEE } from "@/utils/cctp";

type UseDepositForBurnTxArgs = {
  allowance?: bigint;
};

const useDepositForBurnTxArgs = ({ allowance }: UseDepositForBurnTxArgs) => {
  const { address } = useAccount();
  const fromChain = useChainStore.useFromChain();
  const cctpDestinationDomain = useCctpDestinationDomain();
  const token = useFormStore((state) => state.token);
  const amount = useFormStore((state) => state.amount);
  const recipient = useFormStore((state) => state.recipient);

  return useMemo(() => {
    if (
      !address ||
      !fromChain ||
      !token ||
      !amount ||
      allowance === undefined ||
      allowance < amount ||
      !recipient ||
      !isCctp(token)
    ) {
      return;
    }

    return {
      type: "depositForBurn",
      args: {
        to: CCTP_TOKEN_MESSENGER,
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
            CCTP_TRANSFER_MAX_FEE,
            CCTP_MIN_FINALITY_THRESHOLD,
          ],
        }),
        value: 0n,
        chainId: fromChain.id,
      },
    };
  }, [address, allowance, amount, cctpDestinationDomain, fromChain, recipient, token]);
};

export default useDepositForBurnTxArgs;
