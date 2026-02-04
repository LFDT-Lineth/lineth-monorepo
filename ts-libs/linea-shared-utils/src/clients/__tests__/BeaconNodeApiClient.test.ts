import axios from "axios";

import { createLoggerMock, createRetryServiceMock } from "../../__tests__/helpers/factories";
import { PendingDeposit, PendingPartialWithdrawal } from "../../core/client/IBeaconNodeApiClient";
import { IRetryService } from "../../core/services/IRetryService";
import { ILogger } from "../../logging/ILogger";
import { BeaconNodeApiClient } from "../BeaconNodeApiClient";

jest.mock("axios");

const mockedAxios = axios as jest.Mocked<typeof axios>;

// Constants
const RPC_URL = "http://localhost:5051";
const SLOTS_PER_EPOCH = 32;

describe("BeaconNodeApiClient", () => {
  let logger: jest.Mocked<ILogger>;
  let retryService: jest.Mocked<IRetryService>;
  let client: BeaconNodeApiClient;

  beforeEach(() => {
    logger = createLoggerMock();
    retryService = createRetryServiceMock();
    client = new BeaconNodeApiClient(logger, retryService, RPC_URL);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  describe("getPendingPartialWithdrawals", () => {
    const EXPECTED_URL = `${RPC_URL}/eth/v1/beacon/states/head/pending_partial_withdrawals`;

    it("fetch and return pending partial withdrawals", async () => {
      // Arrange
      const rawResponseData = [
        { validator_index: "42", amount: "1234", withdrawable_epoch: "10" },
        { validator_index: "43", amount: "5678", withdrawable_epoch: "11" },
      ];
      const expectedParsedData: PendingPartialWithdrawal[] = [
        { validator_index: 42, amount: 1234n, withdrawable_epoch: 10 },
        { validator_index: 43, amount: 5678n, withdrawable_epoch: 11 },
      ];
      mockedAxios.get.mockResolvedValue({
        data: { execution_optimistic: false, finalized: true, data: rawResponseData },
      });

      // Act
      const result = await client.getPendingPartialWithdrawals();

      // Assert
      expect(result).toEqual(expectedParsedData);
      expect(retryService.retry).toHaveBeenCalledTimes(1);
      expect(retryService.retry.mock.calls[0][0]).toEqual(expect.any(Function));
      expect(mockedAxios.get).toHaveBeenCalledWith(EXPECTED_URL);
    });

    it("return empty array when API responds with no withdrawals", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({
        data: { execution_optimistic: false, finalized: true, data: [] },
      });

      // Act
      const result = await client.getPendingPartialWithdrawals();

      // Assert
      expect(result).toEqual([]);
    });

    it("return undefined when response payload is empty", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({ data: undefined });

      // Act
      const result = await client.getPendingPartialWithdrawals();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("Failed GET request to", EXPECTED_URL);
    });

    it("return undefined when data array is null", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({
        data: { execution_optimistic: false, finalized: true, data: null },
      });

      // Act
      const result = await client.getPendingPartialWithdrawals();

      // Assert
      expect(result).toBeUndefined();
    });
  });

  describe("getPendingDeposits", () => {
    const EXPECTED_URL = `${RPC_URL}/eth/v1/beacon/states/head/pending_deposits`;

    it("fetch and return pending deposits", async () => {
      // Arrange
      const rawResponseData = [
        {
          pubkey: "0x1234",
          withdrawal_credentials: "0xabcd",
          amount: "32000000000",
          signature: "0x5678",
          slot: "100",
        },
        {
          pubkey: "0x5678",
          withdrawal_credentials: "0xef01",
          amount: "32000000000",
          signature: "0x9abc",
          slot: "101",
        },
      ];
      const expectedParsedData: PendingDeposit[] = [
        {
          pubkey: "0x1234",
          withdrawal_credentials: "0xabcd",
          amount: 32000000000,
          signature: "0x5678",
          slot: 100,
        },
        {
          pubkey: "0x5678",
          withdrawal_credentials: "0xef01",
          amount: 32000000000,
          signature: "0x9abc",
          slot: 101,
        },
      ];
      mockedAxios.get.mockResolvedValue({
        data: { execution_optimistic: false, finalized: true, data: rawResponseData },
      });

      // Act
      const result = await client.getPendingDeposits();

      // Assert
      expect(result).toEqual(expectedParsedData);
      expect(retryService.retry).toHaveBeenCalledTimes(1);
      expect(retryService.retry.mock.calls[0][0]).toEqual(expect.any(Function));
      expect(mockedAxios.get).toHaveBeenCalledWith(EXPECTED_URL);
    });

    it("return empty array when API responds with no deposits", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({
        data: { execution_optimistic: false, finalized: true, data: [] },
      });

      // Act
      const result = await client.getPendingDeposits();

      // Assert
      expect(result).toEqual([]);
    });

    it("return undefined when response payload is empty", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({ data: undefined });

      // Act
      const result = await client.getPendingDeposits();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("Failed GET request to", EXPECTED_URL);
    });

    it("return undefined when data array is null", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({
        data: { execution_optimistic: false, finalized: true, data: null },
      });

      // Act
      const result = await client.getPendingDeposits();

      // Assert
      expect(result).toBeUndefined();
    });
  });

  describe("getCurrentEpoch", () => {
    const EXPECTED_URL = `${RPC_URL}/eth/v1/beacon/headers/head`;

    const createBeaconHeaderResponse = (slot: number) => ({
      data: {
        execution_optimistic: false,
        finalized: false,
        data: {
          root: "0xe419f40054c11ebd4973a08ee69c79d3ac65b9c30da4465a4d0b4bcb2718e5a6",
          canonical: true,
          header: {
            message: {
              slot: slot.toString(),
              proposer_index: "337206",
              parent_root: "0xe8bbc35c34d8f6e0d90a0428fe630d816439bdab1963a938700abedc9ffcc89d",
              state_root: "0x8311f8ec859a1af3a8e630f957b6828567364385166a98b36fb7e3c4d165cf55",
              body_root: "0x1576d630e634355976c8236a41df261c6fa29fd3bc053e4d15af234faedd9a1a",
            },
            signature:
              "0xa2c5ddf7700b92216160aae2df3f57981cf3be119a25217a72e68d06eae88c6c290127a1c3a41bd9cf2f2f100bdd960703576e2c7b8c1c6d0f0f296a9139c7971cc6b252add08b9aa7ff16a590d4dfd55027d23042afb8c6befeff9e8dcff7a2",
          },
        },
      },
    });

    it("fetch and return current epoch from beacon head", async () => {
      // Arrange
      const slot = 1892897;
      const expectedEpoch = Math.floor(slot / SLOTS_PER_EPOCH);
      mockedAxios.get.mockResolvedValue(createBeaconHeaderResponse(slot));

      // Act
      const result = await client.getCurrentEpoch();

      // Assert
      expect(result).toBe(expectedEpoch);
      expect(retryService.retry).toHaveBeenCalledTimes(1);
      expect(mockedAxios.get).toHaveBeenCalledWith(EXPECTED_URL);
    });

    it("handle exact epoch boundary slots", async () => {
      // Arrange
      const slot = 3200;
      const expectedEpoch = 100;
      mockedAxios.get.mockResolvedValue(createBeaconHeaderResponse(slot));

      // Act
      const result = await client.getCurrentEpoch();

      // Assert
      expect(result).toBe(expectedEpoch);
    });

    it("floor fractional epoch values correctly", async () => {
      // Arrange
      const slot = 63;
      const expectedEpoch = 1;
      mockedAxios.get.mockResolvedValue(createBeaconHeaderResponse(slot));

      // Act
      const result = await client.getCurrentEpoch();

      // Assert
      expect(result).toBe(expectedEpoch);
    });

    it("return undefined when response payload is empty", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({ data: undefined });

      // Act
      const result = await client.getCurrentEpoch();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("Failed to get slot from response", { url: EXPECTED_URL });
    });

    it("return undefined when data field is missing", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({
        data: {
          execution_optimistic: false,
          finalized: false,
        },
      });

      // Act
      const result = await client.getCurrentEpoch();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("Failed to get slot from response", expect.any(Object));
    });

    it("return undefined when header is missing", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({
        data: {
          execution_optimistic: false,
          finalized: false,
          data: {
            root: "0x1234",
            canonical: true,
          },
        },
      });

      // Act
      const result = await client.getCurrentEpoch();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("Failed to get slot from response", expect.any(Object));
    });

    it("return undefined when message is missing", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({
        data: {
          execution_optimistic: false,
          finalized: false,
          data: {
            root: "0x1234",
            canonical: true,
            header: {
              signature: "0x0000",
            },
          },
        },
      });

      // Act
      const result = await client.getCurrentEpoch();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("Failed to get slot from response", expect.any(Object));
    });

    it("return undefined when slot is null", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({
        data: {
          execution_optimistic: false,
          finalized: false,
          data: {
            root: "0x1234",
            canonical: true,
            header: {
              message: {
                proposer_index: "1",
                parent_root: "0x0000",
                state_root: "0x0000",
                body_root: "0x0000",
              },
              signature: "0x0000",
            },
          },
        },
      });

      // Act
      const result = await client.getCurrentEpoch();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith("Failed to get slot from response", expect.any(Object));
    });

    it("return undefined when slot is invalid (NaN)", async () => {
      // Arrange
      mockedAxios.get.mockResolvedValue({
        data: {
          execution_optimistic: false,
          finalized: false,
          data: {
            root: "0x1234",
            canonical: true,
            header: {
              message: {
                slot: "invalid",
                proposer_index: "1",
                parent_root: "0x0000",
                state_root: "0x0000",
                body_root: "0x0000",
              },
              signature: "0x0000",
            },
          },
        },
      });

      // Act
      const result = await client.getCurrentEpoch();

      // Assert
      expect(result).toBeUndefined();
      expect(logger.error).toHaveBeenCalledWith(`Invalid slot value: invalid from`, EXPECTED_URL);
    });
  });
});
