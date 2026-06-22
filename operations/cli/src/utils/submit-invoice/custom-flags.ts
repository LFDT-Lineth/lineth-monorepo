import { GetCostAndUsageCommandInput } from "@aws-sdk/client-cost-explorer";
import { Flags } from "@oclif/core";

export const awsCostsApiFilters = Flags.custom<GetCostAndUsageCommandInput>({
  parse: (input: string) => {
    try {
      return JSON.parse(input);
    } catch {
      throw new Error("Invalid JSON string for AWS Costs API Filters");
    }
  },
});
