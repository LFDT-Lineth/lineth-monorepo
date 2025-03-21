import { useMemo } from "react";
import { useAccount } from "wagmi";
import { encodeFunctionData, zeroAddress } from "viem";
import MessageService from "@/abis/MessageService.json";
import MessageTransmitterV2 from "@/abis/MessageTransmitterV2.json";
import {
  BridgeTransactionType,
  CctpV2BridgeMessage,
  Chain,
  ChainLayer,
  NativeBridgeMessage,
  TransactionStatus,
} from "@/types";
import { isCctpV2BridgeMessage, isNativeBridgeMessage } from "@/utils/message";

type UseClaimTxArgsProps = {
  status?: TransactionStatus;
  type?: BridgeTransactionType;
  fromChain?: Chain;
  toChain?: Chain;
  args?: NativeBridgeMessage | CctpV2BridgeMessage;
};

const useClaimTxArgs = ({ status, type, fromChain, toChain, args }: UseClaimTxArgsProps) => {
  const { address } = useAccount();

  return useMemo(() => {
    // Missing props
    if (!address || !status || !type || !fromChain || !toChain || !args) return;

    // Must be 'READY_TO_CLAIM' status
    if (status !== TransactionStatus.READY_TO_CLAIM) return;

    let toAddress: `0x${string}`;
    let encodedData: `0x${string}`;

    switch (type) {
      case BridgeTransactionType.ERC20:
      case BridgeTransactionType.ETH:
        if (
          !isNativeBridgeMessage(args) ||
          !args.from ||
          !args.to ||
          args.fee === undefined ||
          args.value === undefined ||
          !args.nonce ||
          !args.calldata ||
          !args.messageHash ||
          (!args.proof && toChain.layer === ChainLayer.L1)
        ) {
          return;
        }

        toAddress = toChain.messageServiceAddress;

        encodedData =
          toChain.layer === ChainLayer.L1
            ? encodeFunctionData({
                abi: MessageService.abi,
                functionName: "claimMessageWithProof",
                args: [
                  {
                    data: args.calldata,
                    fee: args.fee,
                    feeRecipient: zeroAddress,
                    from: args.from,
                    to: args.to,
                    leafIndex: args.proof?.leafIndex,
                    merkleRoot: args.proof?.root,
                    messageNumber: args.nonce,
                    proof: args.proof?.proof,
                    value: args.value,
                  },
                ],
              })
            : encodeFunctionData({
                abi: MessageService.abi,
                functionName: "claimMessage",
                args: [args.from, args.to, args.fee, args.value, zeroAddress, args.calldata, args.nonce],
              });

        break;
      case BridgeTransactionType.USDC:
        if (!isCctpV2BridgeMessage(args) || !args.attestation || !args.message) {
          return;
        }
        toAddress = toChain.cctpMessageTransmitterV2Address;
        encodedData = encodeFunctionData({
          abi: MessageTransmitterV2.abi,
          functionName: "receiveMessage",
          args: [args.message, args.attestation],
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
