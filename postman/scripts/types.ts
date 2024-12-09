import { BytesLike } from "ethers";

export type SendMessageArgs = {
  to: string;
  fee: bigint;
  calldata: BytesLike;
};
