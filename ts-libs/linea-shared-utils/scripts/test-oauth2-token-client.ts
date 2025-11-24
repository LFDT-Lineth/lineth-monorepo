// Script to manually test OAuth2TokenClient against a live server
// pnpm --filter @consensys/linea-shared-utils exec tsx scripts/test-oauth2-token-client.ts
import { ExponentialBackoffRetryService } from "../src";
import { OAuth2TokenClient } from "../src/clients/OAuth2TokenClient";
import { WinstonLogger } from "../src/logging/WinstonLogger";

async function main() {
  const requiredEnvVars = ["TOKEN_URL", "CLIENT_ID", "CLIENT_SECRET", "AUDIENCE"];

  const missing = requiredEnvVars.filter((key) => !process.env[key]);
  if (missing.length > 0) {
    console.error(`Missing required env vars: ${missing.join(", ")}`);
    process.exitCode = 1;
    return;
  }

  const logger = new WinstonLogger("OAuth2TokenClient.integration");
  const retryService = new ExponentialBackoffRetryService(new WinstonLogger(ExponentialBackoffRetryService.name));
  const client = new OAuth2TokenClient(
    logger,
    retryService,
    process.env.TOKEN_URL!,
    process.env.CLIENT_ID!,
    process.env.CLIENT_SECRET!,
    process.env.AUDIENCE!,
    process.env.GRANT_TYPE ?? "client_credentials",
  );

  const firstToken = await client.getBearerToken();
  console.log("First token:", firstToken);

  const secondToken = await client.getBearerToken();
  console.log("Second token (should match first):", secondToken);

  if (firstToken !== secondToken) {
    console.error("Expected cached token but received a different value on second call.");
    process.exitCode = 1;
  }
}

main().catch((err) => {
  console.error("OAuth2TokenClient integration script failed:", err);
  process.exitCode = 1;
});
