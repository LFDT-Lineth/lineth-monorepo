import { Proof } from "./IMerkleTreeService";
import { MessageProps } from "../../../entities/Message";
import {
  IMessageStatusReader,
  IMessageClaimer,
  IRateLimitChecker,
  IContractTransactionErrorParser,
} from "../../../services/contracts/IMessageServiceContract";
import { Address, Hash, MessageSent, Overrides } from "../../../types";

export interface ILineaRollupClient
  extends IMessageStatusReader, IMessageClaimer, IRateLimitChecker, IContractTransactionErrorParser {
  getMessageProof(messageHash: Hash): Promise<Proof>;
  estimateClaimGas(
    message: (MessageSent | MessageProps) & { feeRecipient?: Address; messageBlockNumber?: number },
    opts?: {
      claimViaAddress?: Address;
      overrides?: Overrides;
    },
  ): Promise<bigint>;
}
