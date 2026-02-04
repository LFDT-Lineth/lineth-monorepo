import { useMemo } from "react";

import { encodeFunctionData, padHex, zeroHash } from "viem";

import { TOKEN_MESSENGER_V2_ABI } from "@/abis/TokenMessengerV2";
import { CCTP_MAX_FINALITY_THRESHOLD, CCTP_MIN_FINALITY_THRESHOLD } from "@/constants/cctp";
import { useChainStore } from "@/stores/chainStore";
import { useFormStore } from "@/stores/formStoreProvider";
import { CCTPMode } from "@/types";
import { isNull, isUndefined, isUndefinedOrEmptyString } from "@/utils/misc";
import { isCctp } from "@/utils/tokens";

import { useCctpFee, useCctpDestinationDomain } from "./useCctpUtilHooks";

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
          abi: TOKEN_MESSENGER_V2_ABI,
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
