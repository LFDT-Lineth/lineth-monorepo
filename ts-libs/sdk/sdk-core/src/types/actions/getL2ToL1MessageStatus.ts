import { OnChainMessageStatus } from "../message";
import { Hex } from "../misc";

export type GetL2ToL1MessageStatusParameters<T> = {
  l2Client: T;
  messageHash: Hex;
};

export type GetL2ToL1MessageStatusReturnType = OnChainMessageStatus;
