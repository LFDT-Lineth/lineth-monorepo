import * as dotenv from "dotenv";
import { loadConfigFromEnv } from "./src/application/main/config/loadConfigFromEnv.js";
import { NativeYieldAutomationServiceBootstrap } from "./src/application/main/NativeYieldAutomationServiceBootstrap.js";

dotenv.config();

async function main() {
  const options = loadConfigFromEnv();
  const application = new NativeYieldAutomationServiceBootstrap({
    ...options,
  });
  application.startAllServices();
}

main()
  .then()
  .catch((error) => {
    console.error("", error);
    process.exit(1);
  });

process.on("SIGINT", () => {
  process.exit(0);
});

process.on("SIGTERM", () => {
  process.exit(0);
});
