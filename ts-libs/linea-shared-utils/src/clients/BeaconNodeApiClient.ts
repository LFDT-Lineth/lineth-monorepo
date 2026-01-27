import axios from "axios";

import {
  IBeaconNodeAPIClient,
  PendingPartialWithdrawal,
  PendingPartialWithdrawalResponse,
} from "../core/client/IBeaconNodeApiClient";
import { IRetryService } from "../core/services/IRetryService";
import { ILogger } from "../logging/ILogger";

/**
 * Client for interacting with a Beacon Node API.
 * Provides methods to fetch beacon chain state information with automatic retry support.
 */
export class BeaconNodeApiClient implements IBeaconNodeAPIClient {
  /**
   * Creates a new BeaconNodeApiClient instance.
   *
   * @param {ILogger} logger - The logger instance for logging API requests and responses.
   * @param {IRetryService} retryService - The retry service for handling failed API requests.
   * @param {string} rpcURL - The base URL of the Beacon Node API endpoint.
   */
  constructor(
    private readonly logger: ILogger,
    private readonly retryService: IRetryService,
    private readonly rpcURL: string,
  ) {}

  /**
   * Fetches pending partial withdrawals from the beacon chain state.
   * Makes a GET request to the Beacon Node API with automatic retry on failure.
   *
   * @returns {Promise<PendingPartialWithdrawal[] | undefined>} An array of pending partial withdrawals if successful, undefined if the request fails or returns invalid data.
   */
  async getPendingPartialWithdrawals(): Promise<PendingPartialWithdrawal[] | undefined> {
    const url = `${this.rpcURL}/eth/v1/beacon/states/head/pending_partial_withdrawals`;
    this.logger.debug(`getPendingPartialWithdrawals making GET request to url=${url}`);
    const { data } = await this.retryService.retry(() => axios.get<PendingPartialWithdrawalResponse>(url));
    if (data === undefined || data?.data === undefined) {
      this.logger.error("Failed GET request to", url);
      return undefined;
    }
    const returnVal = data.data;
    this.logger.debug("getPendingPartialWithdrawals return value", { returnVal });
    return returnVal;
  }
}
