import { IContractSignerClient } from "@consensys/linea-shared-utils";
import { parseSignature, serializeTransaction, toRlp, type TransactionSerializable } from "viem";

import { LineaGasFees } from "../../../../core/clients/blockchain/IGasProvider";
import { ITransactionSigner } from "../../../../core/services/ITransactionSigner";
import { TransactionRequest } from "../../../../core/types";

export class ViemTransactionSigner implements ITransactionSigner {
  constructor(
    private readonly signer: IContractSignerClient,
    /** Chain ID used when serializing the dummy transaction for size measurement. Defaults to 1. */
    private readonly chainId: number = 1,
  ) {}

  public async signAndSerialize(tx: TransactionRequest, fees: LineaGasFees): Promise<Uint8Array> {
    const viemTx: TransactionSerializable = {
      chainId: this.chainId,
      nonce: 0,
      to: tx.to,
      value: tx.value ?? 0n,
      data: tx.data,
      maxFeePerGas: fees.maxFeePerGas,
      maxPriorityFeePerGas: fees.maxPriorityFeePerGas,
      gas: fees.gasLimit,
      type: "eip1559",
    };

    const signatureHex = await this.signer.sign(viemTx);
    const signature = parseSignature(signatureHex);
    const serialized = serializeTransaction(viemTx, signature);
    return toRlp(serialized, "bytes");
  }
}
