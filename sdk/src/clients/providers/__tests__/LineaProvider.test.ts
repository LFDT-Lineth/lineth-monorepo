import { Block, FeeData } from "ethers";
import { describe, afterEach, it, beforeEach } from "@jest/globals";
import { LineaProvider } from "..";
import { DEFAULT_MAX_FEE_PER_GAS } from "../../../utils/testing/constants/common";

describe("LineaProvider", () => {
  let lineaProvider: LineaProvider;

  beforeEach(() => {
    lineaProvider = new LineaProvider();
  });

  afterEach(() => {
    jest.resetAllMocks();
  });

  describe("getFees", () => {
    it("should throw an error when getFeeData function does not return `maxPriorityFeePerGas` or `maxFeePerGas` values", async () => {
      jest.spyOn(lineaProvider, "getFeeData").mockResolvedValue({
        maxPriorityFeePerGas: null,
        maxFeePerGas: null,
        gasPrice: 10n,
      } as FeeData);

      await expect(lineaProvider.getFees()).rejects.toThrow("Error getting fee data");
    });

    it("should return `maxPriorityFeePerGas` and `maxFeePerGas` values", async () => {
      jest.spyOn(lineaProvider, "getFeeData").mockResolvedValue({
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        gasPrice: 10n,
      } as FeeData);

      expect(await lineaProvider.getFees()).toStrictEqual({
        maxPriorityFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
        maxFeePerGas: DEFAULT_MAX_FEE_PER_GAS,
      });
    });
  });

  describe("getBlockExtraData", () => {
    it("should return null if getBlock returns null", async () => {
      const blockMocked = null;
      jest.spyOn(lineaProvider, "getBlock").mockResolvedValue(blockMocked);

      expect(await lineaProvider.getBlockExtraData("latest")).toBeNull();
    });

    it("should put requested block number in cache if type of blockNumber param is `number`", async () => {
      const blockMocked: Block = {
        baseFeePerGas: 7n,
        difficulty: 2n,
        extraData:
          "0x0100989680015eb3c80000ea600000000000000000000000000000000000000024997ceb570c667b9c369d351b384ce97dcfe0dda90696fc3b007b8d7160672548a6716cc33ffe0e4004c555a0c7edd9ddc2545a630f2276a2964dcf856e6ab501",
        gasLimit: 61000000n,
        gasUsed: 138144n,
        hash: "0xc53d0f6b65feabf0422bb897d3f5de2c32d57612f88eb4a366d8076a040a715a",
        miner: "0x0000000000000000000000000000000000000000",
        nonce: "0x0000000000000000",
        number: 1635519,
        parentHash: "0xecc7bd0b6d533b13fc65529e5da174062d93f8f426c32929914a375f00a19cc3",
        receiptsRoot: "0x0cfd91942f25f029d50078225749dbd8601dd922713e783e4210568ed45b6cd3",
        stateRoot: "0xad1346e81f574b511c917cd76c5b70f1d6852b871569bedd9ef02160746e3ffa",
        timestamp: 1718030601,
        transactions: [],
        parentBeaconBlockRoot: null,
        blobGasUsed: null,
        excessBlobGas: null,
      } as unknown as Block;
      jest.spyOn(lineaProvider, "getBlock").mockResolvedValue(blockMocked);

      await lineaProvider.getBlockExtraData(10);

      expect(lineaProvider.isCacheValid(10)).toBeTruthy();
    });

    it("should return extraData from cache if requested blockNumber param type is `number`", async () => {
      const blockMocked: Block = {
        baseFeePerGas: 7n,
        difficulty: 2n,
        extraData:
          "0x0100989680015eb3c80000ea600000000000000000000000000000000000000024997ceb570c667b9c369d351b384ce97dcfe0dda90696fc3b007b8d7160672548a6716cc33ffe0e4004c555a0c7edd9ddc2545a630f2276a2964dcf856e6ab501",
        gasLimit: 61000000n,
        gasUsed: 138144n,
        hash: "0xc53d0f6b65feabf0422bb897d3f5de2c32d57612f88eb4a366d8076a040a715a",
        miner: "0x0000000000000000000000000000000000000000",
        nonce: "0x0000000000000000",
        number: 1635519,
        parentHash: "0xecc7bd0b6d533b13fc65529e5da174062d93f8f426c32929914a375f00a19cc3",
        receiptsRoot: "0x0cfd91942f25f029d50078225749dbd8601dd922713e783e4210568ed45b6cd3",
        stateRoot: "0xad1346e81f574b511c917cd76c5b70f1d6852b871569bedd9ef02160746e3ffa",
        timestamp: 1718030601,
        transactions: [],
        parentBeaconBlockRoot: null,
        blobGasUsed: null,
        excessBlobGas: null,
      } as unknown as Block;
      const getBlockSpy = jest.spyOn(lineaProvider, "getBlock").mockResolvedValue(blockMocked);

      await lineaProvider.getBlockExtraData(10);

      expect(lineaProvider.isCacheValid(10)).toBeTruthy();

      await lineaProvider.getBlockExtraData(10);

      expect(getBlockSpy).toHaveBeenCalledTimes(1);
    });

    it("should return block extraData", async () => {
      const blockMocked: Block = {
        baseFeePerGas: 7n,
        difficulty: 2n,
        extraData:
          "0x0100989680015eb3c80000ea600000000000000000000000000000000000000024997ceb570c667b9c369d351b384ce97dcfe0dda90696fc3b007b8d7160672548a6716cc33ffe0e4004c555a0c7edd9ddc2545a630f2276a2964dcf856e6ab501",
        gasLimit: 61000000n,
        gasUsed: 138144n,
        hash: "0xc53d0f6b65feabf0422bb897d3f5de2c32d57612f88eb4a366d8076a040a715a",
        miner: "0x0000000000000000000000000000000000000000",
        nonce: "0x0000000000000000",
        number: 1635519,
        parentHash: "0xecc7bd0b6d533b13fc65529e5da174062d93f8f426c32929914a375f00a19cc3",
        receiptsRoot: "0x0cfd91942f25f029d50078225749dbd8601dd922713e783e4210568ed45b6cd3",
        stateRoot: "0xad1346e81f574b511c917cd76c5b70f1d6852b871569bedd9ef02160746e3ffa",
        timestamp: 1718030601,
        transactions: [],
        parentBeaconBlockRoot: null,
        blobGasUsed: null,
        excessBlobGas: null,
      } as unknown as Block;

      jest.spyOn(lineaProvider, "getBlock").mockResolvedValue(blockMocked);

      expect(await lineaProvider.getBlockExtraData("latest")).toStrictEqual({
        version: 1,
        fixedCost: 10000000000,
        variableCost: 22983624000,
        ethGasPrice: 60000000,
      });
    });
  });
});
