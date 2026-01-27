import { Block, TransactionReceipt, TransactionRequest, TransactionResponse } from "ethers";

import { LineaRollup, LineaRollup__factory } from "../../contracts/typechain";
import { ILineaRollupLogClient } from "../../core/clients/ethereum";
import { IMessageRetriever } from "../../core/clients/IMessageRetriever";
import { IProvider } from "../../core/clients/IProvider";
import { MESSAGE_SENT_EVENT_SIGNATURE } from "../../core/constants";
import { MessageSent } from "../../core/types";
import { isNull } from "../../core/utils";
import { BrowserProvider, Provider } from "../providers";

export class LineaRollupMessageRetriever implements IMessageRetriever<TransactionReceipt> {
  private readonly contract: LineaRollup;

  /**
   * Initializes a new instance of the `LineaRollupMessageRetriever`.
   *
   * @param {IProvider} provider - The provider for interacting with the blockchain.
   * @param {ILineaRollupLogClient} lineaRollupLogClient - An instance of a class implementing the `ILineaRollupLogClient` interface for fetching events from the blockchain.
   * @param {string} contractAddress - The address of the Linea Rollup contract.
   */
  constructor(
    private readonly provider: IProvider<
      TransactionReceipt,
      Block,
      TransactionRequest,
      TransactionResponse,
      Provider | BrowserProvider
    >,
    private readonly lineaRollupLogClient: ILineaRollupLogClient,
    private readonly contractAddress: string,
  ) {
    this.contract = LineaRollup__factory.connect(contractAddress, this.provider);
  }

  /**
   * Retrieves message information by message hash.
   * @param {string} messageHash - The hash of the message sent on L1.
   * @returns {Promise<MessageSent | null>} The message information or null if not found.
   */
  public async getMessageByMessageHash(messageHash: string): Promise<MessageSent | null> {
    const [event] = await this.lineaRollupLogClient.getMessageSentEvents({
      filters: { messageHash },
      fromBlock: 0,
      toBlock: "latest",
    });
    return event ?? null;
  }

  /**
   * Retrieves messages information by the transaction hash.
   * @param {string} transactionHash - The hash of the `sendMessage` transaction on L1.
   * @returns {Promise<MessageSent[] | null>} An array of message information or null if not found.
   */
  public async getMessagesByTransactionHash(transactionHash: string): Promise<MessageSent[] | null> {
    const receipt = await this.provider.getTransactionReceipt(transactionHash);
    if (!receipt) {
      return null;
    }

    const messageSentEvents = await Promise.all(
      receipt.logs
        .filter((log) => log.address === this.contractAddress && log.topics[0] === MESSAGE_SENT_EVENT_SIGNATURE)
        .map((log) => this.contract.interface.parseLog(log))
        .filter((log) => !isNull(log))
        .map((log) => this.getMessageByMessageHash(log!.args._messageHash)),
    );
    return messageSentEvents.filter((log) => !isNull(log)) as MessageSent[];
  }

  /**
   * Retrieves the transaction receipt by message hash.
   * @param {string} messageHash - The hash of the message sent on L1.
   * @returns {Promise<TransactionReceipt | null>} The transaction receipt or null if not found.
   */
  public async getTransactionReceiptByMessageHash(messageHash: string): Promise<TransactionReceipt | null> {
    const [event] = await this.lineaRollupLogClient.getMessageSentEvents({
      filters: { messageHash },
      fromBlock: 0,
      toBlock: "latest",
    });

    if (!event) {
      return null;
    }

    const receipt = await this.provider.getTransactionReceipt(event.transactionHash);
    if (!receipt) {
      return null;
    }
    return receipt;
  }
}
