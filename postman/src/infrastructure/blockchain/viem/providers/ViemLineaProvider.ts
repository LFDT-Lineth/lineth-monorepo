import { getBlockExtraData } from "@lfdt-lineth/sdk-viem";
import { ILogger } from "@lfdt-lineth/shared-utils";
import { BlockNumber, type BlockTag, type PublicClient } from "viem";

import { ViemProvider } from "./ViemProvider";
import { BlockExtraData, ILineaProvider } from "../../../../core/clients/blockchain/linea/ILineaProvider";

export class ViemLineaProvider extends ViemProvider implements ILineaProvider {
  constructor(client: PublicClient, logger: ILogger) {
    super(client, logger);
  }

  public async getBlockExtraData(blockNumber: BlockNumber | BlockTag): Promise<BlockExtraData | null> {
    try {
      if (typeof blockNumber === "string") {
        return await getBlockExtraData(this.client, { blockTag: blockNumber });
      }
      return await getBlockExtraData(this.client, { blockNumber: BigInt(blockNumber) });
    } catch (error) {
      this.logger.warn("Failed to fetch block extra data.", { blockNumber, error });
      return null;
    }
  }
}
