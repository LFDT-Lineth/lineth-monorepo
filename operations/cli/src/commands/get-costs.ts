import { GetCostAndUsageCommandInput } from "@aws-sdk/client-cost-explorer";
import { Command, Flags } from "@oclif/core";
import { addDays } from "date-fns/addDays";
import { formatInTimeZone } from "date-fns-tz";
import { mkdirSync } from "fs";
import { Result } from "neverthrow";
import { join, resolve } from "path";

import { createAwsCostExplorerClient, flattenResultsByTime, getDailyAwsCosts } from "../utils/common/aws.js";
import { writeCsvFile } from "../utils/common/csv.js";
import { isoDate } from "../utils/common/custom-flags.js";
import { generateQueryParameters, getDuneClient, runDuneQuery } from "../utils/common/dune.js";
import { awsCostsApiFilters } from "../utils/submit-invoice/custom-flags.js";

type CostPeriod = {
  startDate: Date;
  endDate: Date;
};

type DailyAwsCost = {
  date: string;
  amount: number;
  estimated: boolean;
};

type OnChainCosts = {
  totalInEth: number;
  columnNames: string[];
  rows: Record<string, unknown>[];
};

export default class GetCosts extends Command {
  static summary =
    "Report on-chain costs (via Dune) and AWS daily costs for a given period, without submitting any transaction.";

  static examples = [
    `<%= config.bin %> <%= command.id %>
      --startDate=2026-06-01
      --endDate=2026-06-10
      --duneApiKey=YOUR_DUNE_KEY
      --duneQueryId=12345
      --awsCostsApiFilters='{"Granularity":"DAILY","Metrics":["AmortizedCost"],"GroupBy":[]}'
      --awsRegion=us-east-1
      --outputDir=./costs-reports
    `,
  ];

  static strict = true;

  static flags = {
    startDate: isoDate({
      description: "Start date of the reporting period (inclusive), in yyyy-MM-dd format (UTC)",
      required: true,
      env: "GET_COSTS_START_DATE",
    }),
    endDate: isoDate({
      description: "End date of the reporting period (inclusive), in yyyy-MM-dd format (UTC)",
      required: true,
      env: "GET_COSTS_END_DATE",
    }),
    duneApiKey: Flags.string({
      description: "Dune API Key",
      required: true,
      env: "GET_COSTS_DUNE_API_KEY",
    }),
    duneQueryId: Flags.integer({
      description: "Dune Query ID returning daily on-chain costs in ETH",
      required: true,
      parse: async (input) => parseInt(input),
      env: "GET_COSTS_DUNE_QUERY_ID",
    }),
    awsCostsApiFilters: awsCostsApiFilters({
      description: "AWS Costs API Filters as JSON string",
      required: true,
      env: "GET_COSTS_AWS_COSTS_API_FILTERS",
    }),
    awsRegion: Flags.string({
      description: "AWS region used by the Cost Explorer client",
      required: false,
      default: "us-east-1",
      env: "GET_COSTS_AWS_REGION",
    }),
    outputDir: Flags.string({
      description: "Directory where the AWS and Dune daily costs CSV files are written",
      required: false,
      default: ".",
      env: "GET_COSTS_OUTPUT_DIR",
    }),
  };

  public async run(): Promise<void> {
    const { flags } = await this.parse(GetCosts);
    const { startDate, endDate, duneApiKey, duneQueryId, awsCostsApiFilters, awsRegion, outputDir } = flags;

    if (endDate.getTime() < startDate.getTime()) {
      this.error(
        `endDate must be greater than or equal to startDate. startDate=${startDate.toISOString()} endDate=${endDate.toISOString()}`,
      );
    }

    const period: CostPeriod = { startDate, endDate };
    const startDateStr = formatInTimeZone(startDate, "UTC", "yyyy-MM-dd");
    const endDateStr = formatInTimeZone(endDate, "UTC", "yyyy-MM-dd");

    this.log(`Reporting costs for period: startDate=${startDateStr} endDate=${endDateStr}`);

    const resolvedOutputDir = resolve(outputDir);
    mkdirSync(resolvedOutputDir, { recursive: true });

    /******************************
        ON-CHAIN COSTS FETCHING
     ******************************/

    const onChainCosts = await this.getOnChainCosts(duneApiKey, duneQueryId, period);

    this.log(
      `Total on-chain costs costsInEth=${onChainCosts.totalInEth} for the period: startDate=${startDateStr} endDate=${endDateStr}`,
    );

    const duneCsvPath = join(resolvedOutputDir, `dune-onchain-daily-costs_${startDateStr}_${endDateStr}.csv`);
    writeCsvFile(duneCsvPath, onChainCosts.columnNames, onChainCosts.rows);
    this.log(`Saved Dune on-chain daily costs to file=${duneCsvPath} rows=${onChainCosts.rows.length}`);

    /******************************
          AWS DAILY COSTS FETCHING
     ******************************/

    const dailyAwsCosts = await this.getDailyAwsCosts(period, awsCostsApiFilters, awsRegion);

    const totalAwsCostsInUsd = dailyAwsCosts.reduce((acc, dailyCost) => acc + dailyCost.amount, 0);

    this.log(`AWS daily costs:`);
    for (const dailyCost of dailyAwsCosts) {
      this.log(
        `type=aws_daily_costs date=${dailyCost.date} amountInUsd=${dailyCost.amount} estimated=${dailyCost.estimated}`,
      );
    }

    this.log(
      `Total AWS costs costsInUsd=${totalAwsCostsInUsd} for the period: startDate=${startDateStr} endDate=${endDateStr}`,
    );

    const awsCsvPath = join(resolvedOutputDir, `aws-daily-costs_${startDateStr}_${endDateStr}.csv`);
    const awsCsvRows = dailyAwsCosts.map((dailyCost) => ({
      date: dailyCost.date,
      amountInUsd: dailyCost.amount,
      estimated: dailyCost.estimated,
    }));
    writeCsvFile(awsCsvPath, ["date", "amountInUsd", "estimated"], awsCsvRows);
    this.log(`Saved AWS daily costs to file=${awsCsvPath} rows=${awsCsvRows.length}`);

    /******************************
              COSTS SUMMARY
     ******************************/

    this.log(
      `type=costs_summary startDate=${startDateStr} endDate=${endDateStr} onChainCostsInEth=${onChainCosts.totalInEth} awsCostsInUsd=${totalAwsCostsInUsd}`,
    );
  }

  /**
   * Fetch on-chain costs for the given period using Dune Analytics.
   * @param duneApiKey Dune API key.
   * @param duneQueryId Dune query ID.
   * @param period Reporting period with start and end dates.
   * @returns The total on-chain costs in ETH plus the raw daily rows and column names.
   */
  private async getOnChainCosts(duneApiKey: string, duneQueryId: number, period: CostPeriod): Promise<OnChainCosts> {
    const duneClient = getDuneClient(duneApiKey);
    const { result } = this.unwrapOrError(
      await runDuneQuery(
        duneClient,
        duneQueryId,
        generateQueryParameters({
          startDate: period.startDate,
          endDate: period.endDate,
        }),
      ),
      "Failed to run Dune query",
    );

    if (!result || !result.rows || result.rows.length === 0) {
      this.error(
        `No Dune query result returned for the specified period. startDate=${period.startDate.toISOString()} endDate=${period.endDate.toISOString()}`,
      );
    }

    const totalInEth = result.rows.reduce((acc, row) => acc + (row.total_costs_per_day as number), 0);
    const columnNames =
      result.metadata?.column_names && result.metadata.column_names.length > 0
        ? result.metadata.column_names
        : Object.keys(result.rows[0]);

    return { totalInEth, columnNames, rows: result.rows };
  }

  /**
   * Fetch AWS daily costs for the given period using the provided AWS Costs API filters.
   * @param period Reporting period with start and end dates.
   * @param awsCostsApiFilters AWS Costs API filters to apply.
   * @param awsRegion AWS region used by the Cost Explorer client.
   * @returns The daily AWS costs for the specified period.
   */
  private async getDailyAwsCosts(
    period: CostPeriod,
    awsCostsApiFilters: GetCostAndUsageCommandInput,
    awsRegion: string,
  ): Promise<DailyAwsCost[]> {
    if (!awsCostsApiFilters.Metrics || awsCostsApiFilters.Metrics.length !== 1) {
      this.error("AWS Costs API Filters must specify exactly one metric.");
    }

    const awsClient = createAwsCostExplorerClient({ region: awsRegion });
    const startDateStr = formatInTimeZone(period.startDate, "UTC", "yyyy-MM-dd");
    const endDateStr = formatInTimeZone(period.endDate, "UTC", "yyyy-MM-dd");
    // The AWS Cost Explorer End date is exclusive, so we add one day to include the requested end date.
    const awsEndDateStr = formatInTimeZone(addDays(period.endDate, 1), "UTC", "yyyy-MM-dd");

    this.log(`Fetching AWS daily costs. startDate=${startDateStr} endDate=${endDateStr}`);

    const { ResultsByTime } = this.unwrapOrError(
      await getDailyAwsCosts(awsClient, {
        ...awsCostsApiFilters,
        TimePeriod: {
          Start: startDateStr,
          End: awsEndDateStr,
        },
      }),
      "Failed to fetch AWS costs",
    );

    if (!Array.isArray(ResultsByTime) || ResultsByTime.length === 0) {
      this.error(`No AWS cost data returned for the specified period. startDate=${startDateStr} endDate=${endDateStr}`);
    }

    return flattenResultsByTime(ResultsByTime, awsCostsApiFilters.Metrics[0]).map((row) => ({
      date: row.date ?? "",
      amount: parseFloat(row.amount),
      estimated: row.estimated,
    }));
  }

  /**
   * Unwrap a Result or throw an error with a custom message.
   * @param result The Result to unwrap.
   * @param message The error message to use if unwrapping fails.
   * @returns The unwrapped value.
   */
  private unwrapOrError<T, E extends Error = Error>(result: Result<T, E>, message: string): T {
    return result.match(
      (value) => value,
      (error) => this.error(`${message}. message=${error.message}`),
    );
  }
}
