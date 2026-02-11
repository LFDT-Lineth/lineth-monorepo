import { wait, serialize, etherToWei } from "@consensys/linea-shared-utils";
import { Account, Chain, Client, Hash, SendTransactionErrorType, Transport } from "viem";
import { getTransactionCount, sendTransaction, SendTransactionParameters } from "viem/actions";
import { EstimateGasParameters } from "viem/linea";

import { estimateLineaGas } from "./gas";
import { createTestLogger } from "../../config/logger";

const logger = createTestLogger();

const FEE_REFRESH_INTERVAL = 10;
const FEE_BUMP_PERCENT = 150n;
const MAX_PENDING_TXS = 5;
const TX_VALUE = etherToWei("0.000001");

type GasFees = Awaited<ReturnType<typeof estimateLineaGas>>;

function bumpFees(fees: GasFees): GasFees {
  return {
    maxPriorityFeePerGas: (fees.maxPriorityFeePerGas * FEE_BUMP_PERCENT) / 100n,
    maxFeePerGas: (fees.maxFeePerGas * FEE_BUMP_PERCENT) / 100n,
    gasLimit: fees.gasLimit,
  };
}

export class TrafficGenerator<
  chain extends Chain | undefined = Chain | undefined,
  account extends Account | undefined = Account | undefined,
> {
  private nonce = 0;
  private fees!: GasFees;
  private txSinceRefresh = 0;
  private isRunning = false;

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
    this.txSinceRefresh = 0;
    this.isRunning = true;

    void this.loop();
  }

  stop(): void {
    this.isRunning = false;
    logger.debug("Stopped generating traffic on L2");
  }

  private async loop(): Promise<void> {
    while (this.isRunning) {
      try {
        await this.refreshFeesIfStale();
        if (!this.isRunning) break;

        await this.unstickIfNeeded();

        const currentNonce = this.nonce++;
        const hash = await this.sendTx(currentNonce, this.fees);
        this.txSinceRefresh++;
        logger.debug(`Traffic tx sent. hash=${hash} nonce=${currentNonce}`);
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
      gas: txFees.gasLimit,
    } as SendTransactionParameters);
  }

  private async refreshFeesIfStale(): Promise<void> {
    if (this.txSinceRefresh >= FEE_REFRESH_INTERVAL) {
      this.fees = await estimateLineaGas(this.publicClient, this.gasParams);
      this.txSinceRefresh = 0;
    }
  }

  private async unstickIfNeeded(): Promise<void> {
    const confirmed = await getTransactionCount(this.publicClient, { address: this.address, blockTag: "latest" });
    const pendingCount = this.nonce - confirmed;

    if (pendingCount <= MAX_PENDING_TXS) return;

    logger.debug(`Stuck txs detected. confirmed=${confirmed} local=${this.nonce} pending=${pendingCount}`);

    this.fees = await estimateLineaGas(this.publicClient, this.gasParams);
    const hash = await this.sendTx(confirmed, bumpFees(this.fees));
    this.txSinceRefresh = 0;

    logger.debug(`Replacement tx sent. nonce=${confirmed} hash=${hash}`);
  }

  private async recover(error: unknown): Promise<void> {
    const e = error as SendTransactionErrorType;
    logger.debug(`Traffic tx error. name=${e.name} message=${e.message}`);

    try {
      this.nonce = await getTransactionCount(this.publicClient, { address: this.address, blockTag: "pending" });
      this.fees = await estimateLineaGas(this.publicClient, this.gasParams);
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
): Promise<() => void> {
  const generator = new TrafficGenerator(walletClient, publicClient, params.pollingInterval);
  await generator.start();
  return () => generator.stop();
}
