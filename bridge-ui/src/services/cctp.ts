import { CctpAttestationApiResponse } from "@/types/cctp";

export async function fetchCctpAttestation(
  transactionHash: string,
  cctpDomain: number,
): Promise<CctpAttestationApiResponse> {
  const response = await fetch(
    `https://iris-api-sandbox.circle.com/v2/messages/${cctpDomain}?transactionHash=${transactionHash}`,
    {
      headers: {
        "Content-Type": "application/json",
      },
    },
  );

  if (!response.ok) {
    throw new Error(`Error in fetchCctpAttestation: transactionHash=${transactionHash} cctpDomain=${cctpDomain}`);
  }

  const data: CctpAttestationApiResponse = await response.json();

  return data;
}
