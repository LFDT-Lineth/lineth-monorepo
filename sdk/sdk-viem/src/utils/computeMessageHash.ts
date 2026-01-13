import { Address, encodeAbiParameters, EncodeAbiParametersErrorType, Hash, keccak256 } from "viem";

export type ComputeMessageHashParameters = {
  from: Address;
  to: Address;
  fee: bigint;
  value: bigint;
  nonce: bigint;
  calldata?: `0x${string}`;
};

export type ComputeMessageHashReturnType = Hash;

export type ComputeMessageHashErrorType = EncodeAbiParametersErrorType;

/**
 * Returns the hash of a message.
 *
 * @returns The details of a message. {@link ComputeMessageHashReturnType}
 * @param client - Client to use
 * @param args - {@link ComputeMessageHashParameters}
 *
 * @example
 * import { computeMessageHash } from '@consensys/linea-sdk-viem'
 *
 * const messageHash = computeMessageHash({
 *   from: '0xSenderAddress',
 *   to: '0xRecipientAddress',
 *   fee: 100_000_000n, // Fee in wei
 *   value: 1_000_000_000_000n,
 *   nonce: 1n,
 *   calldata: '0x',
 * });
 */
export function computeMessageHash(parameters: ComputeMessageHashParameters): ComputeMessageHashReturnType {
  const { from, to, fee, value, nonce, calldata = "0x" } = parameters;
  return keccak256(
    encodeAbiParameters(
      [
        { name: "from", type: "address" },
        { name: "to", type: "address" },
        { name: "fee", type: "uint256" },
        { name: "value", type: "uint256" },
        { name: "nonce", type: "uint256" },
        { name: "calldata", type: "bytes" },
      ],
      [from, to, fee, value, nonce, calldata],
    ),
  );
}
