import { ExtendedMessage } from "../message";
import { Hex } from "../misc";

export type GetMessageByMessageHashParameters = {
  messageHash: Hex;
};

export type GetMessageByMessageHashReturnType<T = bigint> = ExtendedMessage<T>;
