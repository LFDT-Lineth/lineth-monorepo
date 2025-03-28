export enum CctpAttestationMessageStatus {
  PENDING_CONFIRMATIONS = "pending_confirmations",
  COMPLETE = "complete",
}
export type CctpAttestationMessage = {
  attestation: `0x${string}` | "PENDING";
  message: `0x${string}`;
  eventNonce: `0x${string}`;
  cctpVersion: 1 | 2;
  status: CctpAttestationMessageStatus;
};

export type CctpAttestationApiResponse = {
  messages: CctpAttestationMessage[];
};

export type CctpV2ReattestationApiResponse = {
  message: string;
  nonce: `0x${string}`;
};

export type CctpFeeApiResponse = {
  minimumFee: number;
};
