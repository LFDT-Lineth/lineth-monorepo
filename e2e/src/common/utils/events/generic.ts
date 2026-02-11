import { Abi, Account, BlockNumber, BlockTag, Chain, Client, ContractEventName, Transport } from "viem";
import { GetContractEventsParameters, GetContractEventsReturnType, getContractEvents } from "viem/actions";

import { awaitUntil, AwaitUntilTimeoutError } from "../wait";
import { WaitForEventsTimeoutError } from "./errors";

export async function getEvents<
  chain extends Chain | undefined,
  account extends Account | undefined,
  const Tabi extends Abi | readonly unknown[],
  eventName extends ContractEventName<Tabi> | undefined = undefined,
  strict extends boolean | undefined = undefined,
  fromBlock extends BlockNumber | BlockTag | undefined = undefined,
  toBlock extends BlockNumber | BlockTag | undefined = undefined,
>(
  client: Client<Transport, chain, account>,
  params: GetContractEventsParameters<Tabi, eventName, strict, fromBlock, toBlock> & {
    criteria?: (
      events: GetContractEventsReturnType<Tabi, eventName, strict, fromBlock, toBlock>,
    ) => Promise<GetContractEventsReturnType<Tabi, eventName, strict, fromBlock, toBlock>>;
  },
): Promise<GetContractEventsReturnType<Tabi, eventName, strict, fromBlock, toBlock>> {
  const { criteria, ...rest } = params;
  const events = await getContractEvents(client, rest);

  if (criteria) {
    return await criteria(events);
  }

  return events;
}

export async function waitForEvents<
  chain extends Chain | undefined,
  account extends Account | undefined,
  const Tabi extends Abi | readonly unknown[],
  eventName extends ContractEventName<Tabi> | undefined = undefined,
  strict extends boolean | undefined = undefined,
  fromBlock extends BlockNumber | BlockTag | undefined = undefined,
  toBlock extends BlockNumber | BlockTag | undefined = undefined,
>(
  client: Client<Transport, chain, account>,
  params: GetContractEventsParameters<Tabi, eventName, strict, fromBlock, toBlock> & {
    pollingIntervalMs?: number;
    timeoutMs?: number;
    criteria?: (
      events: GetContractEventsReturnType<Tabi, eventName, strict, fromBlock, toBlock>,
    ) => Promise<GetContractEventsReturnType<Tabi, eventName, strict, fromBlock, toBlock>>;
  },
): Promise<GetContractEventsReturnType<Tabi, eventName, strict, fromBlock, toBlock>> {
  try {
    return await awaitUntil(
      async () => await getEvents(client, params),
      (a: GetContractEventsReturnType<Tabi, eventName, strict, fromBlock, toBlock>) => a.length > 0,
      {
        pollingIntervalMs: params.pollingIntervalMs ?? 500,
        ...(params.timeoutMs !== undefined ? { timeoutMs: params.timeoutMs } : {}),
      },
    );
  } catch (err) {
    if (err instanceof AwaitUntilTimeoutError) {
      throw new WaitForEventsTimeoutError({
        timeoutMs: params.timeoutMs ?? 2 * 60 * 1000,
        address: params.address,
        eventName: params.eventName,
        args: params.args,
        fromBlock: params.fromBlock,
        toBlock: params.toBlock,
      });
    }

    throw err;
  }
}
