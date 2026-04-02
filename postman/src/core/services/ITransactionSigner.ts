import { LineaGasFees } from "../clients/blockchain/IGasProvider";
import { TransactionRequest } from "../types";

export interface ITransactionSigner {
  /** Sign and RLP-encode a transaction, returning raw bytes for size measurement. */
  signAndSerialize(tx: TransactionRequest, fees: LineaGasFees): Promise<Uint8Array>;
}
