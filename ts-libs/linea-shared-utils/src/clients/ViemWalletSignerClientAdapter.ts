import {
  Account,
  Address,
  Chain,
  createWalletClient,
  Hex,
  http,
  parseTransaction,
  serializeSignature,
  TransactionSerializable,
  WalletClient,
} from "viem";
import { privateKeyToAccount, privateKeyToAddress } from "viem/accounts";

import { IContractSignerClient } from "../core/client/IContractSignerClient";
import { ILogger } from "../logging/ILogger";

/**
 * Adapter that wraps viem's WalletClient to provide contract signing functionality.
 * Uses a private key to create a wallet account and sign transactions.
 */
export class ViemWalletSignerClientAdapter implements IContractSignerClient {
  private readonly account: Account;
  private readonly address: Address;
  private readonly wallet: WalletClient;

  /**
   * Creates a new ViemWalletSignerClientAdapter instance.
   *
   * @param {ILogger} logger - The logger instance for logging signing operations.
   * @param {string} rpcUrl - The RPC URL for the blockchain network.
   * @param {Hex} privateKey - The private key in hex format to use for signing.
   * @param {Chain} chain - The blockchain chain configuration.
   */
  constructor(
    private readonly logger: ILogger,
    rpcUrl: string,
    privateKey: Hex,
    chain: Chain,
  ) {
    this.account = privateKeyToAccount(privateKey);
    this.address = privateKeyToAddress(privateKey);
    this.wallet = createWalletClient({
      account: this.account,
      chain,
      transport: http(rpcUrl),
    });
  }

  /**
   * Signs a transaction using the wallet's private key.
   * Strips any existing signature fields from the transaction before signing.
   *
   * @param {TransactionSerializable} tx - The transaction to sign (signature fields will be removed if present).
   * @returns {Promise<Hex>} The serialized signature as a hex string.
   * @throws {Error} If the signature components (r, s, yParity) are missing after signing.
   */
  async sign(tx: TransactionSerializable): Promise<Hex> {
    this.logger.debug("sign started...", { tx });
    // Remove any signature fields if they exist on the object
    // 'as any' required to avoid enforcing strict structural validation
    // Fine because we are only removing fields, not depending on them existing
    // Practical way to strip off optional keys from a union type
    const { r: r_void, s: s_void, v: v_void, yParity: yParity_void, ...unsigned } = tx as any; // eslint-disable-line @typescript-eslint/no-explicit-any
    void r_void;
    void s_void;
    void v_void;
    void yParity_void;

    const serializedSignedTx = await this.wallet.signTransaction({ ...unsigned });
    const parsedTx = parseTransaction(serializedSignedTx);
    this.logger.debug("sign", { parsedTx });
    const { r, s, yParity } = parsedTx;
    // TODO - Better error handling
    if (!r || !s || yParity === undefined) {
      this.logger.error("sign - r, s or yParity missing");
      throw new Error("sign - r, s or yParity missing");
    }

    const signatureHex = serializeSignature({
      r,
      s,
      yParity,
    });

    this.logger.debug(`sign completed signatureHex=${signatureHex}`);
    return signatureHex;
  }

  /**
   * Gets the Ethereum address associated with the wallet's private key.
   *
   * @returns {Address} The Ethereum address derived from the private key.
   */
  getAddress(): Address {
    return this.address;
  }
}
