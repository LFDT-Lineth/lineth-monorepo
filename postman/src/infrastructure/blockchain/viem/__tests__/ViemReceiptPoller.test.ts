import { ILogger } from "@consensys/linea-shared-utils";
import { describe, it, beforeEach, expect } from "@jest/globals";
import { mock } from "jest-mock-extended";

import { ITransactionProvider } from "../../../../core/clients/blockchain/IProvider";
import { BaseError } from "../../../../core/errors";
import { TransactionReceipt } from "../../../../core/types";
import { TEST_TRANSACTION_HASH } from "../../../../utils/testing/constants";
import { ViemReceiptPoller } from "../ViemReceiptPoller";

const generateReceipt = (): TransactionReceipt => ({
  hash: TEST_TRANSACTION_HASH,
  blockNumber: 100,
  status: "success",
  gasUsed: 50_000n,
  gasPrice: 100_000_000_000n,
  logs: [],
});

describe("ViemReceiptPoller", () => {
  const provider = mock<ITransactionProvider>();
  const logger = mock<ILogger>();
  let poller: ViemReceiptPoller;

  beforeEach(() => {
    jest.resetAllMocks();
    poller = new ViemReceiptPoller(provider, logger);
  });

  it("returns receipt immediately when available", async () => {
    const receipt = generateReceipt();
    jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(receipt);

    const result = await poller.poll(TEST_TRANSACTION_HASH, 5000, 100);

    expect(result).toBe(receipt);
    expect(provider.getTransactionReceipt).toHaveBeenCalledTimes(1);
  });

  it("polls until receipt is found", async () => {
    const receipt = generateReceipt();
    jest
      .spyOn(provider, "getTransactionReceipt")
      .mockResolvedValueOnce(null)
      .mockResolvedValueOnce(null)
      .mockResolvedValueOnce(receipt);

    const result = await poller.poll(TEST_TRANSACTION_HASH, 5000, 0);

    expect(result).toBe(receipt);
    expect(provider.getTransactionReceipt).toHaveBeenCalledTimes(3);
  });

  it("throws BaseError when timeout is reached", async () => {
    jest.spyOn(provider, "getTransactionReceipt").mockResolvedValue(null);

    await expect(poller.poll(TEST_TRANSACTION_HASH, 0, 0)).rejects.toThrow(BaseError);
  });
});
