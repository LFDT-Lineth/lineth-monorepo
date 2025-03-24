import { useMemo } from "react";
import { useAccount } from "wagmi";
import { encodeFunctionData, padHex, zeroHash } from "viem";
import { useFormStore, useChainStore } from "@/stores";
import { isCctp } from "@/utils/tokens";
import { useCctpFee, useCctpDestinationDomain } from "./useCctpUtilHooks";
import { CCTP_MIN_FINALITY_THRESHOLD } from "@/constants";

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
  const fee = useCctpFee();

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
            CCTP_MIN_FINALITY_THRESHOLD,
          ],
        }),
        value: 0n,
        chainId: fromChain.id,
      },
    };
  }, [address, allowance, amount, fee, cctpDestinationDomain, fromChain, recipient, token]);
};

export default useDepositForBurnTxArgs;
