import { QueryParameter, DuneClient, ResultsResponse, DuneError } from "@duneanalytics/client-sdk";
import { err, ok, Result } from "neverthrow";

export function getDuneClient(duneApiKey: string): DuneClient {
  return new DuneClient(duneApiKey);
}

export function generateQueryParameter(key: string, value: string | number | Date): QueryParameter {
  if (typeof value === "string") {
    return QueryParameter.text(key, value);
  }

  if (value instanceof Date) {
    return QueryParameter.date(key, value.toISOString().slice(0, 19).replace("T", " "));
  }

  if (typeof value === "number" && Number.isInteger(value)) {
    return QueryParameter.number(key, value);
  }

  throw new Error(`Unsupported query parameter type for key: ${key}`);
}

export function generateQueryParameters(params: Record<string, string | number | Date>): QueryParameter[] {
  return Object.entries(params).map(([key, value]) => generateQueryParameter(key, value));
}

export async function runDuneQuery(
  duneClient: DuneClient,
  duneQueryId: number,
  params: QueryParameter[] = [],
): Promise<Result<ResultsResponse, DuneError>> {
  try {
    const response = await duneClient.runQuery({
      queryId: duneQueryId,
      query_parameters: params,
    });
    return ok(response);
  } catch (error) {
    return err(error as DuneError);
  }
}
