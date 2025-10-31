import {
  CostExplorerClient,
  CostExplorerClientConfigType,
  GetCostAndUsageCommand,
  GetCostAndUsageCommandInput,
  GetCostAndUsageCommandOutput,
} from "@aws-sdk/client-cost-explorer";
import { err, ok, Result } from "neverthrow";

export function createAwsCostExplorerClient(config: CostExplorerClientConfigType): CostExplorerClient {
  return new CostExplorerClient(config);
}

export async function getDailyAwsCosts(
  client: CostExplorerClient,
  params: GetCostAndUsageCommandInput,
): Promise<Result<GetCostAndUsageCommandOutput, Error>> {
  try {
    const command = new GetCostAndUsageCommand(params);
    const response = await client.send(command);
    return ok(response);
  } catch (error) {
    return err(error as Error);
  }
}
