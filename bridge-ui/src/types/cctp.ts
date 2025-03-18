export type CctpAttestationMessageStatus = "pending_confirmations" | "complete";

type CctpAttestationMessage = {
  attestation: `0x${string}`;
  message: `0x${string}`;
  eventNonce: `0x${string}`;
  cctpVersion: 1 | 2;
  status: CctpAttestationMessageStatus;
};

export type CctpAttestationApiResponse = {
  messages: CctpAttestationMessage[];
};
