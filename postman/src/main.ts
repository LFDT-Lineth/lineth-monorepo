import { WinstonLogger } from "@consensys/linea-shared-utils";
import * as dotenv from "dotenv";

import { loadPostmanOptionsFromEnv } from "./application/postman/app/config/envLoader";
import { PostmanApp } from "./application/postman/app/PostmanApp";

dotenv.config();

const bootstrapLogger = new WinstonLogger("Bootstrap");
let app: PostmanApp | undefined;

async function main() {
  const options = loadPostmanOptionsFromEnv();
  app = new PostmanApp(options);
  await app.start();
}

async function shutdown() {
  bootstrapLogger.info("Shutdown signal received — draining services.");
  try {
    await app?.stop();
    // eslint-disable-next-line no-empty
  } catch {}
  process.exit(0);
}

main().catch((error: unknown) => {
  const safeError = error instanceof Error ? { message: error.message } : { message: String(error) };
  bootstrapLogger.error("Fatal startup error.", safeError);
  process.exit(1);
});

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);
