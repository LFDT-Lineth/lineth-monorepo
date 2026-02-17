import { wait, serialize, etherToWei } from "@consensys/linea-shared-utils";
import { Account, Chain, Client, Hash, SendTransactionErrorType, Transport } from "viem";
import { getTransactionCount, sendTransaction, SendTransactionParameters } from "viem/actions";
import { EstimateGasParameters } from "viem/linea";

import { estimateLineaGas } from "./gas";
import { createTestLogger } from "../../config/logger";

const logger = createTestLogger();

const FEE_REFRESH_INTERVAL = 10;
const FEE_BUMP_PERCENT = 115n;
const MAX_PENDING_TXS = 3;
const TX_VALUE = etherToWei("0.000001");

/**
 * Maximum multiplier above the last-estimated base fee before the compounding
 * bump is capped and fees are re-estimated from chain. Prevents runaway gas
 * prices under sustained congestion.
 */
const MAX_FEE_MULTIPLIER = 10n;

type GasFees = Awaited<ReturnType<typeof estimateLineaGas>>;

function bumpFees(fees: GasFees): GasFees {
  return {
    maxPriorityFeePerGas: (fees.maxPriorityFeePerGas * FEE_BUMP_PERCENT) / 100n,
    maxFeePerGas: (fees.maxFeePerGas * FEE_BUMP_PERCENT) / 100n,
    gas: fees.gas,
  };
}

export class TrafficGenerator<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
> {
  private nonce = 0;
  private fees!: GasFees;
  private baseFees!: GasFees;
  private txSinceRefresh = 0;
  private isRunning = false;
  private loopPromise: Promise<void> | null = null;

  private readonly address: `0x${string}`;
  private readonly gasParams: EstimateGasParameters;

  constructor(
    private readonly walletClient: Client<Transport, chain, account>,
    private readonly publicClient: Client<Transport, chain, account>,
    private readonly pollingInterval = 1_000,
  ) {
    const walletAccount = walletClient.account;
    if (!walletAccount) throw new Error("Wallet client does not have an associated account");

    this.address = walletAccount.address;
    this.gasParams = { account: this.address, to: this.address, value: TX_VALUE } as EstimateGasParameters;
  }

  async start(): Promise<void> {
    this.nonce = await getTransactionCount(this.publicClient, { address: this.address, blockTag: "pending" });
    this.fees = await estimateLineaGas(this.publicClient, this.gasParams);
    this.baseFees = this.fees;
    this.txSinceRefresh = 0;
    this.isRunning = true;

    this.loopPromise = this.loop();
  }

  async stop(): Promise<void> {
    this.isRunning = false;
    if (this.loopPromise) {
      await this.loopPromise;
      this.loopPromise = null;
    }
    logger.debug("Stopped generating traffic on L2");
  }

  private async loop(): Promise<void> {
    while (this.isRunning) {
      try {
        await this.refreshFeesIfStale();
        if (!this.isRunning) break;

        const wasStuck = await this.unstickIfNeeded();
        if (!wasStuck) {
          const currentNonce = this.nonce++;
          const hash = await this.sendTx(currentNonce, this.fees);
          this.txSinceRefresh++;
          logger.debug(`Traffic tx sent. hash=${hash} nonce=${currentNonce}`);
        }
      } catch (error) {
        await this.recover(error);
      }

      await wait(this.pollingInterval);
    }
  }

  private async sendTx(txNonce: number, txFees: GasFees): Promise<Hash> {
    return sendTransaction(this.walletClient, {
      to: this.address,
      value: TX_VALUE,
      type: "eip1559",
      nonce: txNonce,
      maxPriorityFeePerGas: txFees.maxPriorityFeePerGas,
      maxFeePerGas: txFees.maxFeePerGas,
      gas: txFees.gas,
    } as SendTransactionParameters);
  }

  private async refreshFeesIfStale(): Promise<void> {
    if (this.txSinceRefresh >= FEE_REFRESH_INTERVAL) {
      this.fees = await estimateLineaGas(this.publicClient, this.gasParams);
      this.baseFees = this.fees;
      this.txSinceRefresh = 0;
    }
  }

  private async unstickIfNeeded(): Promise<boolean> {
    const confirmed = await getTransactionCount(this.publicClient, { address: this.address, blockTag: "latest" });
    const pendingCount = this.nonce - confirmed;

    if (pendingCount <= MAX_PENDING_TXS) return false;

    logger.debug(`Stuck txs detected. confirmed=${confirmed} local=${this.nonce} pending=${pendingCount}`);

    // Bump from current fees so each successive unstick attempt is progressively
    // higher. A fresh estimateLineaGas would return the same values on a stable
    // chain, producing a "Known transaction" rejection.
    this.fees = bumpFees(this.fees);

    // Cap compounding: if fees grew beyond MAX_FEE_MULTIPLIER of the last
    // chain-estimated base, re-estimate from chain and bump once from there.
    if (this.fees.maxFeePerGas > this.baseFees.maxFeePerGas * MAX_FEE_MULTIPLIER) {
      logger.debug(
        `Fee cap reached. Re-estimating from chain. maxFeePerGas=${this.fees.maxFeePerGas} cap=${this.baseFees.maxFeePerGas * MAX_FEE_MULTIPLIER}`,
      );
      this.fees = await estimateLineaGas(this.publicClient, this.gasParams);
      this.baseFees = this.fees;
      this.fees = bumpFees(this.fees);
    }

    const hash = await this.sendTx(confirmed, this.fees);
    this.txSinceRefresh = 0;

    logger.debug(`Replacement tx sent. nonce=${confirmed} hash=${hash}`);
    return true;
  }

  private async recover(error: unknown): Promise<void> {
    const e = error as SendTransactionErrorType;
    logger.debug(`Traffic tx error. name=${e.name} message=${e.message}`);

    try {
      this.nonce = await getTransactionCount(this.publicClient, { address: this.address, blockTag: "pending" });
      this.fees = await estimateLineaGas(this.publicClient, this.gasParams);
      this.baseFees = this.fees;
      this.txSinceRefresh = 0;
    } catch (recoveryError) {
      logger.debug(`Recovery failed. error=${serialize(recoveryError)}`);
    }
  }
}

/**
 * Convenience wrapper that preserves the original API.
 * Creates a TrafficGenerator, starts it, and returns a stop function.
 */
export async function sendTransactionsToGenerateTrafficWithInterval<
  chain extends Chain | undefined,
  account extends Account | undefined,
>(
  walletClient: Client<Transport, chain, account>,
  publicClient: Client<Transport, chain, account>,
  params: { pollingInterval?: number },
): Promise<() => Promise<void>> {
  const generator = new TrafficGenerator(walletClient, publicClient, params.pollingInterval);
  await generator.start();
  return () => generator.stop();
}
