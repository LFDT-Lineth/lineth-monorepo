import { Account, Address, Chain, Client, ReadContractErrorType, Transport } from "viem";
import { readContract } from "viem/actions";

import { NEXT_MESSAGE_NUMBER_ABI } from "../abis";

export type GetNextMessageNonceParameters = {
  rollupAddress: Address;
};

export type GetNextMessageNonceReturnType = bigint;

export type GetNextMessageNonceErrorType = ReadContractErrorType;

export async function getNextMessageNonce<chain extends Chain | undefined, _account extends Account | undefined>(
  client: Client<Transport, chain, _account>,
  parameters: GetNextMessageNonceParameters,
): Promise<GetNextMessageNonceReturnType> {
  const { rollupAddress } = parameters;

  return readContract(client, {
    address: rollupAddress,
    abi: NEXT_MESSAGE_NUMBER_ABI,
    functionName: "nextMessageNumber",
  });
}
