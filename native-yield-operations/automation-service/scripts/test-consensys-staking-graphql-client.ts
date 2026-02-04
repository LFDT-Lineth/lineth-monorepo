/**
 * Manual integration runner for ConsensysStakingApiClient.
 *
 * Example usage:
 * GRAPHQL_ENDPOINT=https://example/graphql \
 * TOKEN_URL=... \
 * CLIENT_ID=... \
 * CLIENT_SECRET=... \
 * AUDIENCE=... \
 * BEACON_NODE_RPC_URL=https://example/beacon \
 * pnpm --filter @consensys/linea-native-yield-automation-service exec tsx scripts/test-consensys-staking-graphql-client.ts
 */

import {
  BeaconNodeApiClient,
  WinstonLogger,
  OAuth2TokenClient,
  ExponentialBackoffRetryService,
} from "@consensys/linea-shared-utils";
import { ConsensysStakingApiClient } from "../src/clients/ConsensysStakingApiClient.js";
import { createApolloClient } from "../src/utils/createApolloClient.js";

// private readonly apolloClient: ApolloClient,

async function main() {
  const requiredEnvVars = [
    "GRAPHQL_ENDPOINT",
    "BEACON_NODE_RPC_URL",
    "TOKEN_URL",
    "CLIENT_ID",
    "CLIENT_SECRET",
    "AUDIENCE",
  ];
  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }

  const retryService = new ExponentialBackoffRetryService(new WinstonLogger(ExponentialBackoffRetryService.name));
  const beaconClient = new BeaconNodeApiClient(
    new WinstonLogger("BeaconNodeApiClient.integration"),
    retryService,
    process.env.BEACON_NODE_RPC_URL!,
  );

  const tokenClient = new OAuth2TokenClient(
    new WinstonLogger("OAuth2TokenClient.integration"),
    retryService,
    process.env.TOKEN_URL!,
    process.env.CLIENT_ID!,
    process.env.CLIENT_SECRET!,
    process.env.AUDIENCE!,
  );

  const apolloClient = createApolloClient(tokenClient, process.env.GRAPHQL_ENDPOINT!);
  const consensysStakingClient = new ConsensysStakingApiClient(
    new WinstonLogger("ConsensysStakingApiClient.integration"),
    retryService,
    apolloClient,
    beaconClient,
  );

  try {
    const validators = await consensysStakingClient.getValidatorsForWithdrawalRequestsAscending();
    if (validators === undefined) {
      console.error("Failed getValidatorsForWithdrawalRequestsAscending");
      throw "Failed getValidatorsForWithdrawalRequestsAscending";
    }
    console.log(`Fetched ${validators.length} validators with pending withdrawals.`);
    console.log(validators);
    const totalPendingWei = consensysStakingClient.getTotalPendingPartialWithdrawalsWei(validators);
    console.log(`Total pending partial withdrawals (wei): ${totalPendingWei.toString()}`);
  } catch (err) {
    console.error("ConsensysStakingApiClient integration script failed:", err);
    process.exitCode = 1;
  }

  try {
    const validators = await consensysStakingClient.getExitingValidators();
    if (validators === undefined) {
      console.error("Failed getExitingValidators");
      throw "Failed getExitingValidators";
    }
    console.log(`Fetched ${validators.length} exiting validators.`);
    console.log(validators);
  } catch (err) {
    console.error("ConsensysStakingApiClient integration script failed:", err);
    process.exitCode = 1;
  }

  try {
    const validators = await consensysStakingClient.getExitedValidators();
    if (validators === undefined) {
      console.error("Failed getExitedValidators");
      throw "Failed getExitedValidators";
    }
    console.log(`Fetched ${validators.length} exited validators.`);
    console.log(validators);
  } catch (err) {
    console.error("ConsensysStakingApiClient integration script failed:", err);
    process.exitCode = 1;
  }
}

main();
