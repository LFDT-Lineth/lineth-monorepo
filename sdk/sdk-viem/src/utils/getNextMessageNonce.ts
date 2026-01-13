import { Account, Address, Chain, Client, ReadContractErrorType, Transport } from "viem";
import { readContract } from "viem/actions";

export type GetNextMessageNonceParameters = {
  lineaRollupAddress: Address;
};

export type GetNextMessageNonceReturnType = bigint;

export type GetNextMessageNonceErrorType = ReadContractErrorType;

export async function getNextMessageNonce<chain extends Chain | undefined, _account extends Account | undefined>(
  client: Client<Transport, chain, _account>,
  parameters: GetNextMessageNonceParameters,
): Promise<GetNextMessageNonceReturnType> {
  const { lineaRollupAddress } = parameters;

  return readContract(client, {
    address: lineaRollupAddress,
    abi: [
      {
        inputs: [],
        name: "nextMessageNumber",
        outputs: [
          {
            internalType: "uint256",
            name: "",
            type: "uint256",
          },
        ],
        stateMutability: "view",
        type: "function",
      },
    ],
    functionName: "nextMessageNumber",
  });
}
