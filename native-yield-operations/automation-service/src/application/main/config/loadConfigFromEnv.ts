import { configSchema } from "./config.schema.js";
import { toClientConfig } from "./config.js";

/**
 * Loads and validates configuration from environment variables.
 * Parses the environment object using the config schema, validates it, and converts it to client configuration.
 * If validation fails, outputs pretty-ish error output for CI/boot logs and exits the process with code 1.
 *
 * @param {NodeJS.ProcessEnv} [envObj=process.env] - The environment object to parse. Defaults to process.env.
 * @returns {ReturnType<typeof toClientConfig>} The validated and converted client configuration object.
 * @throws {never} Exits the process with code 1 if configuration validation fails.
 */
export function loadConfigFromEnv(envObj: NodeJS.ProcessEnv = process.env) {
  const parsed = configSchema.safeParse(envObj);
  if (!parsed.success) {
    // pretty-ish error output for CI/boot logs
    console.error("❌ Invalid configuration:");
    console.error(JSON.stringify(parsed.error.format(), null, 2));
    process.exit(1);
  }
  return toClientConfig(parsed.data);
}
