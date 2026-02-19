import dotenv from "dotenv";
import * as fs from "fs";
import * as path from "path";
import { fileURLToPath } from "url";

import { loadConfigFromEnv } from "./src/application/main/config/index.js";
import { LidoGovernanceMonitorBootstrap } from "./src/application/main/LidoGovernanceMonitorBootstrap.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Load environment variables
dotenv.config();

async function main(): Promise<void> {
  console.log("Starting Lido Governance Monitor...");

  // Load configuration
  const config = loadConfigFromEnv(process.env as Record<string, string>);

  // Load system prompt from file
  const projectRoot = path.basename(__dirname) === "dist" ? path.resolve(__dirname, "..") : __dirname;
  const systemPromptPath = path.join(projectRoot, "src/prompts/risk-assessment-system.md");
  const systemPrompt = fs.readFileSync(systemPromptPath, "utf-8");

  // Create and start the application
  const app = LidoGovernanceMonitorBootstrap.create(config, systemPrompt);

  // Handle graceful shutdown
  const shutdown = async (signal: string): Promise<void> => {
    console.log(`Received ${signal}, shutting down...`);
    await app.stop();
    process.exit(0);
  };

  process.on("SIGINT", () => void shutdown("SIGINT"));
  process.on("SIGTERM", () => void shutdown("SIGTERM"));

  // Start the application (one-shot execution)
  try {
    await app.start();
    console.log("Lido Governance Monitor one-shot execution completed successfully.");
  } finally {
    // Always cleanup to prevent process hang
    await app.stop();
  }

  // Exit cleanly after one-shot execution
  process.exit(0);
}

main().catch((error) => {
  console.error("Fatal error:", error);
  process.exit(1);
});
