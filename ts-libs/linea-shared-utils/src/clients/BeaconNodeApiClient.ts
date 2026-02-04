import axios from "axios";

import {
  IBeaconNodeAPIClient,
  PendingDeposit,
  PendingPartialWithdrawal,
  RawPendingDeposit,
  RawPendingPartialWithdrawal,
  BeaconHeadResponse,
} from "../core/client/IBeaconNodeApiClient";
import { IRetryService } from "../core/services/IRetryService";
import { ILogger } from "../logging/ILogger";
import { slotToEpoch } from "../utils/blockchain";

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
   * Parses a raw pending partial withdrawal response from the API into a typed object.
   *
   * @param {RawPendingPartialWithdrawal} raw - The raw API response with string values.
   * @returns {PendingPartialWithdrawal} The parsed withdrawal with correct types.
   */
  private parsePendingPartialWithdrawal(raw: RawPendingPartialWithdrawal): PendingPartialWithdrawal {
    return {
      validator_index: parseInt(raw.validator_index, 10),
      amount: BigInt(raw.amount),
      withdrawable_epoch: parseInt(raw.withdrawable_epoch, 10),
    };
  }

  /**
   * Parses a raw pending deposit response from the API into a typed object.
   *
   * @param {RawPendingDeposit} raw - The raw API response with string values.
   * @returns {PendingDeposit} The parsed deposit with correct types.
   */
  private parsePendingDeposit(raw: RawPendingDeposit): PendingDeposit {
    return {
      pubkey: raw.pubkey,
      withdrawal_credentials: raw.withdrawal_credentials,
      amount: parseInt(raw.amount, 10),
      signature: raw.signature,
      slot: parseInt(raw.slot, 10),
    };
  }

  /**
   * Fetches pending partial withdrawals from the beacon chain state.
   * Makes a GET request to the Beacon Node API with automatic retry on failure.
   *
   * @returns {Promise<PendingPartialWithdrawal[] | undefined>} An array of pending partial withdrawals if successful, undefined if the request fails or returns invalid/null data.
   */
  async getPendingPartialWithdrawals(): Promise<PendingPartialWithdrawal[] | undefined> {
    const url = `${this.rpcURL}/eth/v1/beacon/states/head/pending_partial_withdrawals`;
    this.logger.debug(`getPendingPartialWithdrawals making GET request to url=${url}`);
    const { data } = await this.retryService.retry(() =>
      axios.get<{ execution_optimistic: boolean; finalized: boolean; data: RawPendingPartialWithdrawal[] }>(url),
    );
    if (data === undefined || data?.data === undefined) {
      this.logger.error("Failed GET request to", url);
      return undefined;
    }
    if (data.data === null) {
      this.logger.info(`getPendingPartialWithdrawals succeeded, pendingWithdrawalCount=0`);
      this.logger.debug("getPendingPartialWithdrawals return value", { returnVal: undefined });
      return undefined;
    }
    const returnVal = data.data.map((raw) => this.parsePendingPartialWithdrawal(raw));
    this.logger.info(`getPendingPartialWithdrawals succeeded, pendingWithdrawalCount=${returnVal.length}`);
    this.logger.debug("getPendingPartialWithdrawals return value", { returnVal });
    return returnVal;
  }

  /**
   * Fetches pending deposits from the beacon chain state.
   * Makes a GET request to the Beacon Node API with automatic retry on failure.
   *
   * @returns {Promise<PendingDeposit[] | undefined>} An array of pending deposits if successful, undefined if the request fails or returns invalid/null data.
   */
  async getPendingDeposits(): Promise<PendingDeposit[] | undefined> {
    const url = `${this.rpcURL}/eth/v1/beacon/states/head/pending_deposits`;
    this.logger.debug(`getPendingDeposits making GET request to url=${url}`);
    const { data } = await this.retryService.retry(() =>
      axios.get<{ execution_optimistic: boolean; finalized: boolean; data: RawPendingDeposit[] }>(url),
    );
    if (data === undefined || data?.data === undefined) {
      this.logger.error("Failed GET request to", url);
      return undefined;
    }
    if (data.data === null) {
      this.logger.info(`getPendingDeposits succeeded, pendingDepositCount=0`);
      this.logger.debug("getPendingDeposits return value", { returnVal: undefined });
      return undefined;
    }
    const returnVal = data.data.map((raw) => this.parsePendingDeposit(raw));
    this.logger.info(`getPendingDeposits succeeded, pendingDepositCount=${returnVal.length}`);
    this.logger.debug("getPendingDeposits return value", { returnVal });
    return returnVal;
  }

  /**
   * Fetches the current epoch from the beacon chain head.
   * Makes a GET request to the Beacon Node API with automatic retry on failure.
   *
   * @returns {Promise<number | undefined>} The current epoch number if successful, undefined if the request fails or returns invalid data.
   */
  async getCurrentEpoch(): Promise<number | undefined> {
    const url = `${this.rpcURL}/eth/v1/beacon/headers/head`;
    this.logger.debug(`getCurrentEpoch making GET request to url=${url}`);
    const { data } = await this.retryService.retry(() => axios.get<BeaconHeadResponse>(url));
    const slotString = data?.data?.header?.message?.slot;
    if (slotString === undefined || slotString === null) {
      this.logger.error("Failed to get slot from response", { url });
      return undefined;
    }
    const slot = parseInt(slotString, 10);
    if (isNaN(slot)) {
      this.logger.error(`Invalid slot value: ${slotString} from`, url);
      return undefined;
    }
    const epoch = slotToEpoch(slot);
    this.logger.info(`getCurrentEpoch succeeded, epoch=${epoch}, slot=${slot}`);
    this.logger.debug("getCurrentEpoch return value", { epoch, slot });
    return epoch;
  }
}
