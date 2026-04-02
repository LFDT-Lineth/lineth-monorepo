import { MessageProps } from "../../../entities/Message";
import {
  IMessageStatusReader,
  IMessageClaimer,
  IRateLimitChecker,
  IContractTransactionErrorParser,
} from "../../../services/contracts/IMessageServiceContract";
import { Address, Hex, MessageSent, Overrides } from "../../../types";
import { LineaGasFees } from "../IGasProvider";

export interface IL2MessageServiceClient
  extends IMessageStatusReader, IMessageClaimer, IRateLimitChecker, IContractTransactionErrorParser {
  encodeClaimMessageTransactionData(message: MessageProps & { feeRecipient?: Address }): Hex;
  estimateClaimGasFees(
    message: (MessageSent | MessageProps) & { feeRecipient?: Address },
    opts?: {
      claimViaAddress?: Address;
      overrides?: Overrides;
    },
  ): Promise<LineaGasFees>;
  getContractAddress(): Address;
}
