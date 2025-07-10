export { OnChainMessageStatus, type MessageProof, type Message } from "@consensys/linea-sdk-core";

export { deposit } from "./actions/deposit";
export type { DepositParameters, DepositReturnType } from "./actions/deposit";

export { withdraw } from "./actions/withdraw";
export type { WithdrawParameters, WithdrawReturnType } from "./actions/withdraw";

export { getBlockExtraData } from "./actions/getBlockExtraData";
export type { GetBlockExtraDataParameters, GetBlockExtraDataReturnType } from "./actions/getBlockExtraData";

export { getL1ToL2MessageStatus } from "./actions/getL1ToL2MessageStatus";
export type {
  GetL1ToL2MessageStatusParameters,
  GetL1ToL2MessageStatusReturnType,
} from "./actions/getL1ToL2MessageStatus";

export { getL2ToL1MessageStatus } from "./actions/getL2ToL1MessageStatus";
export type {
  GetL2ToL1MessageStatusParameters,
  GetL2ToL1MessageStatusReturnType,
} from "./actions/getL2ToL1MessageStatus";

export { getMessageByMessageHash } from "./actions/getMessageByMessageHash";
export type {
  GetMessageByMessageHashParameters,
  GetMessageByMessageHashReturnType,
} from "./actions/getMessageByMessageHash";

export { getMessageProof } from "./actions/getMessageProof";
export type { GetMessageProofParameters, GetMessageProofReturnType } from "./actions/getMessageProof";

export { getMessageSentEvents } from "./actions/getMessageSentEvents";
export type { GetMessageSentEventsParameters, GetMessageSentEventsReturnType } from "./actions/getMessageSentEvents";

export { getMessagesByTransactionHash } from "./actions/getMessagesByTransactionHash";
export type {
  GetMessagesByTransactionHashParameters,
  GetMessagesByTransactionHashReturnType,
} from "./actions/getMessagesByTransactionHash";

export { getTransactionReceiptByMessageHash } from "./actions/getTransactionReceiptByMessageHash";
export type {
  GetTransactionReceiptByMessageHashParameters,
  GetTransactionReceiptByMessageHashReturnType,
} from "./actions/getTransactionReceiptByMessageHash";

export { claimOnL1 } from "./actions/claimOnL1";
export type { ClaimOnL1Parameters, ClaimOnL1ReturnType } from "./actions/claimOnL1";

export { claimOnL2 } from "./actions/claimOnL2";
export type { ClaimOnL2Parameters, ClaimOnL2ReturnType } from "./actions/claimOnL2";

export { publicActionsL1 } from "./decorators/publicL1";
export type { PublicActionsL1 } from "./decorators/publicL1";

export { publicActionsL2 } from "./decorators/publicL2";
export type { PublicActionsL2 } from "./decorators/publicL2";

export { walletActionsL1 } from "./decorators/walletL1";
export type { WalletActionsL1 } from "./decorators/walletL1";

export { walletActionsL2 } from "./decorators/walletL2";
export type { WalletActionsL2 } from "./decorators/walletL2";
