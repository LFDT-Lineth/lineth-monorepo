import axios from "axios";
import { mock } from "jest-mock-extended";
import { BeaconNodeApiClient } from "../BeaconNodeApiClient";
import { ILogger } from "../../logging/ILogger";
import { IRetryService } from "../../core/services/IRetryService";
import { PendingPartialWithdrawal } from "../../core/client/IBeaconNodeApiClient";

jest.mock("axios");

const mockedAxios = axios as jest.Mocked<typeof axios>;

describe("BeaconNodeApiClient", () => {
  const rpcURL = "http://localhost:5051";
  let logger: jest.Mocked<ILogger>;
  let retryService: jest.Mocked<IRetryService>;
  let client: BeaconNodeApiClient;

  beforeEach(() => {
    logger = {
      name: "test",
      info: jest.fn(),
      error: jest.fn(),
      warn: jest.fn(),
      debug: jest.fn(),
    };
    retryService = mock<IRetryService>();
    retryService.retry.mockImplementation(async (fn) => fn());
    mockedAxios.get.mockReset();

    client = new BeaconNodeApiClient(logger, retryService, rpcURL);
  });

  afterEach(() => {
    jest.clearAllMocks();
  });

  it("fetches and returns pending partial withdrawals", async () => {
    const responseData: PendingPartialWithdrawal[] = [
      { validator_index: 42, amount: 1234n, withdrawable_epoch: 10 },
      { validator_index: 43, amount: 5678n, withdrawable_epoch: 11 },
    ];
    mockedAxios.get.mockResolvedValue({ data: { data: responseData } });

    const result = await client.getPendingPartialWithdrawals();

    const expectedUrl = `${rpcURL}/eth/v1/beacon/states/head/pending_partial_withdrawals`;
    expect(result).toEqual(responseData);
    expect(retryService.retry).toHaveBeenCalledTimes(1);
    expect(retryService.retry.mock.calls[0][0]).toEqual(expect.any(Function));
    expect(mockedAxios.get).toHaveBeenCalledWith(expectedUrl);
    expect(logger.debug).toHaveBeenNthCalledWith(
      1,
      `getPendingPartialWithdrawals making GET request to url=${expectedUrl}`,
    );
    expect(logger.debug).toHaveBeenNthCalledWith(2, "getPendingPartialWithdrawals return value", {
      returnVal: responseData,
    });
    expect(logger.error).not.toHaveBeenCalled();
  });

  it("logs an error and returns undefined when response payload is empty", async () => {
    mockedAxios.get.mockResolvedValue({ data: undefined });

    const result = await client.getPendingPartialWithdrawals();

    const expectedUrl = `${rpcURL}/eth/v1/beacon/states/head/pending_partial_withdrawals`;
    expect(result).toBeUndefined();
    expect(logger.error).toHaveBeenCalledWith("Failed GET request to", expectedUrl);
    expect(logger.debug).toHaveBeenCalledTimes(1);
  });

  it("returns an empty array when the API responds with no withdrawals", async () => {
    mockedAxios.get.mockResolvedValue({ data: { data: [] } });

    const result = await client.getPendingPartialWithdrawals();

    expect(result).toEqual([]);
    expect(logger.error).not.toHaveBeenCalled();
    expect(logger.debug).toHaveBeenCalledTimes(2);
  });
});
