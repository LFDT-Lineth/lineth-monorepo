import * as lineaSdkViem from "@consensys/linea-sdk-viem";
import { describe, it, expect, beforeEach, afterEach } from "@jest/globals";
import { mock } from "jest-mock-extended";
import { PublicClient } from "viem";

import { mockLogger } from "../../../../../utils/testing/mocks";
import { ViemLineaProvider } from "../ViemLineaProvider";

jest.mock("@consensys/linea-sdk-viem", () => ({
  getBlockExtraData: jest.fn(),
}));

describe("ViemLineaProvider", () => {
  let provider: ViemLineaProvider;
  const client = mock<PublicClient>();
  const logger = mockLogger();
  const getBlockExtraDataMock = lineaSdkViem.getBlockExtraData as jest.Mock;

  beforeEach(() => {
    provider = new ViemLineaProvider(client, logger);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe("getBlockExtraData", () => {
    it("should call getBlockExtraData with blockNumber when number is provided", async () => {
      const expectedData = { version: 1, fixedCost: 100n, variableCost: 200n, ethGasPrice: 300n };
      getBlockExtraDataMock.mockResolvedValue(expectedData);

      const result = await provider.getBlockExtraData(42n);

      expect(getBlockExtraDataMock).toHaveBeenCalledWith(client, { blockNumber: 42n });
      expect(result).toEqual(expectedData);
    });

    it("should call getBlockExtraData with blockTag when string is provided", async () => {
      const expectedData = { version: 1, fixedCost: 100n, variableCost: 200n, ethGasPrice: 300n };
      getBlockExtraDataMock.mockResolvedValue(expectedData);

      const result = await provider.getBlockExtraData("latest");

      expect(getBlockExtraDataMock).toHaveBeenCalledWith(client, { blockTag: "latest" });
      expect(result).toEqual(expectedData);
    });

    it("should return null and log warning when getBlockExtraData throws", async () => {
      const error = new Error("RPC error");
      getBlockExtraDataMock.mockRejectedValue(error);

      const result = await provider.getBlockExtraData(42n);

      expect(result).toBeNull();
      expect(logger.warn).toHaveBeenCalledWith("Failed to fetch block extra data.", {
        blockNumber: 42n,
        error,
      });
    });
  });
});
