import { OnChainMessageStatus } from "../message";
import { Hex } from "../misc";

export type GetL1ToL2MessageStatusParameters = {
  messageHash: Hex;
};

export type GetL1ToL2MessageStatusReturnType = OnChainMessageStatus;
