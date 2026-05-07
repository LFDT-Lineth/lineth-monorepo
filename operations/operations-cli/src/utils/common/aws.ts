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

export function flattenResultsByTime(
  resultsByTime: GetCostAndUsageCommandOutput["ResultsByTime"],
  metric = "AmortizedCost",
) {
  if (!resultsByTime) {
    return [];
  }

  const rows = [];

  for (const item of resultsByTime) {
    const { Start } = item.TimePeriod ?? {};

    const hasGroups = Array.isArray(item.Groups) && item.Groups.length > 0;

    if (item.Total && !hasGroups) {
      const data = item.Total[metric];
      rows.push({
        date: Start,
        amount: data?.Amount ?? "0",
        estimated: item.Estimated ?? false,
      });
      continue;
    }

    if (hasGroups) {
      for (const group of item.Groups!) {
        if (!group.Metrics) continue;

        const data = group.Metrics[metric];

        rows.push({
          date: Start,
          groupKeys: group.Keys?.join(" / ") ?? "",
          amount: data?.Amount ?? "0",
          estimated: item.Estimated ?? false,
        });
      }
    }
  }

  return rows;
}
