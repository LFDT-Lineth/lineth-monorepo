import { ILogger } from "@consensys/linea-shared-utils";

import { ITransactionProvider } from "../../../core/clients/blockchain/IProvider";
import { BaseError } from "../../../core/errors";
import { IReceiptPoller } from "../../../core/services/IReceiptPoller";
import { Hash, TransactionReceipt } from "../../../core/types";
import { wait } from "../../../core/utils/shared";

export class ViemReceiptPoller implements IReceiptPoller {
  constructor(
    private readonly provider: ITransactionProvider,
    private readonly logger: ILogger,
  ) {}

  public async poll(transactionHash: Hash, timeout: number, interval: number): Promise<TransactionReceipt> {
    this.logger.debug("Polling for transaction receipt.", { transactionHash, timeout, interval });

    const deadline = Date.now() + timeout;
    while (Date.now() < deadline) {
      const receipt = await this.provider.getTransactionReceipt(transactionHash);
      if (receipt) return receipt;
      await wait(interval);
    }

    this.logger.warn("Transaction receipt not found after polling timeout.", { transactionHash, timeout });
    throw new BaseError(`Transaction receipt not found after polling. transactionHash=${transactionHash}`);
  }
}
