import { IContractSignerClient } from "@consensys/linea-shared-utils";
import { parseSignature, serializeTransaction } from "viem";
import { toAccount } from "viem/accounts";

import { UnsupportedOperationError } from "../../../../core/errors";

import type { LocalAccount } from "viem";

/**
 * Bridges an IContractSignerClient into a viem LocalAccount so it can be used
 * to create a WalletClient for transaction submission.
 *
 * Only signTransaction is supported — signMessage and signTypedData will throw
 * because the underlying signer is purpose-built for raw transaction signing.
 */
export function contractSignerToViemAccount(signer: IContractSignerClient): LocalAccount {
  const address = signer.getAddress();

  return toAccount({
    address,
    async signTransaction(transaction) {
      const signatureHex = await signer.sign(transaction);
      const signature = parseSignature(signatureHex);
      return serializeTransaction(transaction, signature);
    },
    async signMessage() {
      throw new UnsupportedOperationError(
        "signMessage",
        `IContractSignerClient at ${address} only supports transaction signing`,
      );
    },
    async signTypedData() {
      throw new UnsupportedOperationError(
        "signTypedData",
        `IContractSignerClient at ${address} only supports transaction signing`,
      );
    },
  });
}
