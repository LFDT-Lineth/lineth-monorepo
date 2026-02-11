import { serialize } from "@consensys/linea-shared-utils";
import { Abi, Address, BlockNumber, BlockTag, ContractEventArgs, ContractEventName } from "viem";

export class WaitForEventsTimeoutError<
  const Tabi extends Abi | readonly unknown[],
  eventName extends ContractEventName<Tabi> | undefined = undefined,
  fromBlock extends BlockNumber | BlockTag | undefined = undefined,
  toBlock extends BlockNumber | BlockTag | undefined = undefined,
> extends Error {
  constructor(args: {
    timeoutMs: number;
    address?: Address | Address[] | undefined;
    eventName?: eventName;
    args?:
      | ContractEventArgs<Tabi, eventName extends ContractEventName<Tabi> ? eventName : ContractEventName<Tabi>>
      | undefined;
    fromBlock?: fromBlock;
    toBlock?: toBlock;
  }) {
    super(
      [
        `Timed out after ${args.timeoutMs}ms waiting for contract events`,
        args.address && `address=${args.address}`,
        args.eventName && `event=${args.eventName}`,
        args.args && `args=${serialize(args.args)}`,
        `fromBlock=${args.fromBlock ?? 0}`,
        `toBlock=${args.toBlock ?? "latest"}`,
      ]
        .filter(Boolean)
        .join(" "),
    );
    this.name = "WaitForEventsTimeoutError";
  }
}
