import { getConnection } from "./connection.js";

export const getLastBlockTimestamp = async (): Promise<bigint> => {
  const { networkHelpers } = await getConnection();
  return BigInt(await networkHelpers.time.latest());
};

export const setFutureTimestampForNextBlock = async (secondsInTheFuture: number | bigint = 1): Promise<bigint> => {
  const { networkHelpers } = await getConnection();
  const lastBlockTimestamp: number = await networkHelpers.time.latest();
  const futureTimestamp: bigint = BigInt(lastBlockTimestamp) + BigInt(secondsInTheFuture);
  await networkHelpers.time.setNextBlockTimestamp(futureTimestamp);
  return futureTimestamp;
};
