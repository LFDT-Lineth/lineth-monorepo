import { expect } from "chai";
import {
  type Abi,
  type AbiEvent,
  type AbiItemArgs,
  type AbiItemName,
  type AbiParameter,
  type Address,
  type ContractErrorArgs,
  type ContractErrorName,
  type ContractEventName,
  type Hash,
  type TransactionReceipt,
  decodeEventLog,
  parseEventLogs,
  zeroAddress,
} from "viem";

import { getConnection } from "./connection.js";

/**
 * Assertion helpers for the suite, implemented with Hardhat's `viem.assertions` (`hardhat-viem-assertions`) on the
 * connection the suite runs on (`getConnection()`, so `useFreshChain()` suites assert on their own chain).
 *
 * Expected args are positional, type-checked against the contract's ABI and compared like `viem.assertions` does:
 * strictly (bigints for integers wider than 48 bits, `Hex` values, structs as viem decodes them), except addresses,
 * which compare case-insensitively. Any position may be a `(value) => boolean` predicate, e.g. `anyValue` from
 * `@nomicfoundation/hardhat-viem-assertions/predicates`.
 */

/** A contract as the helpers see it, with the ABI widened to `Abi` so shared helpers can name errors by string. */
export type AssertionTarget = { abi: Abi; address: Address };

/** Per-position expected value or predicate, as `viem.assertions` accepts them. */
type WithPredicates<TArgs> = TArgs extends readonly unknown[]
  ? number extends TArgs["length"]
    ? unknown[]
    : { -readonly [K in keyof TArgs]: TArgs[K] | ((value: TArgs[K]) => boolean) }
  : unknown[];

export type ExpectedErrorArgs<TAbi extends Abi, TErrorName extends ContractErrorName<TAbi>> = WithPredicates<
  ContractErrorArgs<TAbi, TErrorName>
>;

export type ExpectedEventArgs<TAbi extends Abi, TEventName extends ContractEventName<TAbi>> =
  TEventName extends AbiItemName<TAbi> ? WithPredicates<AbiItemArgs<TAbi, TEventName>> : unknown[];

/** The `viem.assertions` methods the helpers call, seen with string names and untyped args (the helpers type them). */
type Assertions = {
  revertWith(call: Promise<unknown>, reason: string): Promise<void>;
  revertWithCustomError(call: Promise<unknown>, contract: AssertionTarget, errorName: string): Promise<void>;
  revertWithCustomErrorWithArgs(
    call: Promise<unknown>,
    contract: AssertionTarget,
    errorName: string,
    args: readonly unknown[],
  ): Promise<void>;
  emit(txHash: Hash | Promise<Hash>, contract: AssertionTarget, eventName: string): Promise<void>;
  emitWithArgs(
    txHash: Hash | Promise<Hash>,
    contract: AssertionTarget,
    eventName: string,
    args: readonly unknown[],
  ): Promise<void>;
};

async function getAssertions(): Promise<Assertions> {
  const { viem } = await getConnection();
  return viem.assertions;
}

/**
 * Asserts that `asyncCall` (a write, read, simulation or deployment) reverts with the custom error `errorName` of
 * `contract`, and with `errorArgs` when given. `contract` only needs an ABI: deployment reverts can pass an artifact.
 */
export async function expectRevertWithCustomError<const TAbi extends Abi, TErrorName extends ContractErrorName<TAbi>>(
  contract: { abi: TAbi; address?: Address },
  asyncCall: Promise<unknown>,
  errorName: TErrorName,
  errorArgs?: ExpectedErrorArgs<TAbi, TErrorName>,
): Promise<void> {
  const assertions = await getAssertions();
  // Revert assertions decode with the ABI only; the address is never read.
  const target: AssertionTarget = { abi: contract.abi, address: contract.address ?? zeroAddress };
  if (errorArgs === undefined || errorArgs.length === 0) {
    await assertions.revertWithCustomError(asyncCall, target, errorName);
  } else {
    await assertions.revertWithCustomErrorWithArgs(asyncCall, target, errorName, errorArgs);
  }
}

/** Asserts that `asyncCall` reverts with the reason string `reason` (`require(..., reason)` / `revert(reason)`). */
export async function expectRevertWithReason(asyncCall: Promise<unknown>, reason: string): Promise<void> {
  const assertions = await getAssertions();
  await assertions.revertWith(asyncCall, reason);
}

/**
 * Asserts that the transaction emitted `eventName` from `contract.address`, with `eventArgs` when given (passes if
 * any emitted occurrence matches).
 */
export async function expectEvent<const TAbi extends Abi, TEventName extends ContractEventName<TAbi>>(
  contract: { abi: TAbi; address: Address },
  asyncCall: Hash | Promise<Hash>,
  eventName: TEventName,
  eventArgs?: ExpectedEventArgs<TAbi, TEventName>,
): Promise<void> {
  const assertions = await getAssertions();
  if (eventArgs === undefined || eventArgs.length === 0) {
    await assertions.emit(asyncCall, contract, eventName);
  } else {
    await assertions.emitWithArgs(asyncCall, contract, eventName, eventArgs);
  }
}

/** Asserts several events of one transaction (each as `expectEvent` with args). */
export async function expectEvents(
  contract: AssertionTarget,
  asyncCall: Hash | Promise<Hash>,
  events: { name: string; args: readonly unknown[] }[],
): Promise<void> {
  const assertions = await getAssertions();
  const hash = await asyncCall;
  for (const event of events) {
    await assertions.emitWithArgs(hash, contract, event.name, event.args);
  }
}

/** Asserts that the transaction emitted no `eventName` event from `contract` (no `viem.assertions` equivalent). */
export async function expectNoEvent(
  contract: AssertionTarget,
  asyncCall: Hash | Promise<Hash>,
  eventName: string,
): Promise<void> {
  const { viem } = await getConnection();
  const publicClient = await viem.getPublicClient();
  const receipt = await publicClient.waitForTransactionReceipt({ hash: await asyncCall });
  const ownLogs = receipt.logs.filter((log) => log.address.toLowerCase() === contract.address.toLowerCase());
  expect(
    parseEventLogs({ abi: contract.abi, eventName, logs: ownLogs }),
    `unexpected "${eventName}" event`,
  ).to.have.length(0);
}

/**
 * Decodes the log at `logIndex` of a receipt with `contract.abi` and checks its name and args (as a set, positionally
 * normalized). For events emitted by another address than the ABI holder (e.g. a proxy called through a forwarder),
 * which `viem.assertions.emit*` filter out.
 */
export function expectEventDirectFromReceiptData(
  contract: { abi: Abi },
  transactionReceipt: TransactionReceipt,
  expectedEventName: string,
  expectedEventArgs: unknown[] = [],
  logIndex: number = 0,
) {
  const log = transactionReceipt.logs[logIndex];
  expect(log, `no log at index ${logIndex}`).to.not.equal(undefined);
  const event = decodeEventLog({ abi: contract.abi, data: log!.data, topics: log!.topics });
  expect(event.eventName).to.equal(expectedEventName);
  const inputs = eventInputs(contract.abi, expectedEventName);
  expect(normalizeArgs(event.args ?? [], inputs)).to.have.deep.members(normalizeArgs(expectedEventArgs, inputs));
}

function eventInputs(abi: Abi, eventName: string): readonly AbiParameter[] {
  const event = abi.find((item): item is AbiEvent => item.type === "event" && item.name === eventName);
  return event?.inputs ?? [];
}

/** Converts decoded or expected args (array or named object) to a positional, normalized array. */
function normalizeArgs(args: unknown, inputs: readonly AbiParameter[]): unknown[] {
  let list: readonly unknown[] = [];
  if (Array.isArray(args)) {
    list = args;
  } else if (isRecord(args)) {
    list = inputs.map((input, index) => args[input.name ?? String(index)]);
  }
  return list.map((value, index) => normalizeValue(value, inputs[index]));
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/** Numbers and bigints compare as decimal strings, hex case-insensitively, structs as positional arrays. */
function normalizeValue(value: unknown, param?: AbiParameter): unknown {
  if (typeof value === "number" || typeof value === "bigint") {
    return BigInt(value).toString();
  }
  if (typeof value === "string") {
    return value.startsWith("0x") ? value.toLowerCase() : value;
  }
  if (param?.type.startsWith("tuple") && "components" in param) {
    const isArray = param.type.endsWith("]");
    if (isArray && Array.isArray(value)) {
      const element: AbiParameter = { ...param, type: param.type.slice(0, param.type.lastIndexOf("[")) };
      return value.map((item) => normalizeValue(item, element));
    }
    return normalizeArgs(value, param.components);
  }
  if (Array.isArray(value)) {
    const element: AbiParameter | undefined =
      param && param.type.endsWith("]")
        ? { ...param, type: param.type.slice(0, param.type.lastIndexOf("[")) }
        : undefined;
    return value.map((item) => normalizeValue(item, element));
  }
  return value;
}
