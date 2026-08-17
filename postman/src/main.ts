import { WinstonLogger } from "@lfdt-lineth/shared-utils";
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
  bootstrapLogger.error("Fatal startup error.", error);
  process.exit(1);
});

process.on("SIGINT", shutdown);
process.on("SIGTERM", shutdown);
