import { useMemo } from "react";

import { encodeFunctionData, zeroAddress } from "viem";
import { useConnection } from "wagmi";

import { MESSAGE_SERVICE_ABI } from "@/abis/MessageService";
import { MESSAGE_TRANSMITTER_V2_ABI } from "@/abis/MessageTransmitterV2";
import {
  BridgeTransactionType,
  CctpV2BridgeMessage,
  Chain,
  ChainLayer,
  NativeBridgeMessage,
  TransactionStatus,
} from "@/types";
import { isCctpV2BridgeMessage, isNativeBridgeMessage } from "@/utils/message";
import { isUndefined, isUndefinedOrEmptyString } from "@/utils/misc";

type UseClaimTxArgsProps = {
  status?: TransactionStatus;
  type?: BridgeTransactionType;
  fromChain?: Chain;
  toChain?: Chain;
  args?: NativeBridgeMessage | CctpV2BridgeMessage;
};

const useClaimTxArgs = ({ status, type, fromChain, toChain, args }: UseClaimTxArgsProps) => {
  const { address } = useConnection();

  return useMemo(() => {
    // Missing props
    if (
      isUndefinedOrEmptyString(address) ||
      isUndefined(status) ||
      isUndefined(type) ||
      isUndefined(fromChain) ||
      isUndefined(toChain) ||
      isUndefined(args)
    )
      return;

    // Must be 'READY_TO_CLAIM' status
    if (status !== TransactionStatus.READY_TO_CLAIM) return;

    let toAddress: `0x${string}`;
    let encodedData: `0x${string}`;

    switch (type) {
      case BridgeTransactionType.ERC20:
      case BridgeTransactionType.ETH:
        if (
          !isNativeBridgeMessage(args) ||
          isUndefinedOrEmptyString(args.from) ||
          isUndefinedOrEmptyString(args.to) ||
          isUndefined(args.fee) ||
          isUndefined(args.value) ||
          isUndefined(args.nonce) ||
          args.nonce === 0n ||
          isUndefinedOrEmptyString(args.calldata) ||
          isUndefinedOrEmptyString(args.messageHash) ||
          (isUndefined(args.proof) && toChain.layer === ChainLayer.L1)
        ) {
          return;
        }

        toAddress = toChain.messageServiceAddress;

        encodedData =
          toChain.layer === ChainLayer.L1
            ? encodeFunctionData({
                abi: MESSAGE_SERVICE_ABI,
                functionName: "claimMessageWithProof",
                args: [
                  {
                    data: args.calldata as `0x{string}`,
                    fee: args.fee,
                    feeRecipient: zeroAddress,
                    from: args.from,
                    to: args.to,
                    leafIndex: args.proof?.leafIndex as number,
                    merkleRoot: args.proof?.root as `0x{string}`,
                    messageNumber: args.nonce,
                    proof: args.proof?.proof as `0x{string}`[],
                    value: args.value,
                  },
                ],
              })
            : encodeFunctionData({
                abi: MESSAGE_SERVICE_ABI,
                functionName: "claimMessage",
                args: [
                  args.from,
                  args.to,
                  args.fee,
                  args.value,
                  zeroAddress,
                  args.calldata as `0x{string}`,
                  args.nonce,
                ],
              });

        break;
      case BridgeTransactionType.USDC:
        if (
          !isCctpV2BridgeMessage(args) ||
          isUndefinedOrEmptyString(args.attestation) ||
          isUndefinedOrEmptyString(args.message)
        ) {
          return;
        }
        toAddress = toChain.cctpMessageTransmitterV2Address;
        encodedData = encodeFunctionData({
          abi: MESSAGE_TRANSMITTER_V2_ABI,
          functionName: "receiveMessage",
          args: [args.message as `0x{string}`, args.attestation as `0x{string}`],
        });
        break;
      default:
        return;
    }

    return {
      type: "claim",
      args: {
        to: toAddress,
        data: encodedData,
        value: 0n,
        chainId: toChain.id,
      },
    };
  }, [address, status, type, fromChain, toChain, args]);
};

export default useClaimTxArgs;
