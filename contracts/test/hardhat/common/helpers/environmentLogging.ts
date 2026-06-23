import { expect } from "chai";

import {
  formatEnvVarForLog,
  formatEnvVarValueForMessage,
  isEnvVarContext,
  isSensitiveEnvVar,
} from "../../../../common/helpers/envVarLogging";

/** Synthetic names — each triggers a denylist pattern; not real project env vars. */
const SENSITIVE_ENV_VAR_EXAMPLES = [
  "EXAMPLE_PRIVATE_KEY",
  "EXAMPLE_RPC_URL",
  "EXAMPLE_L1_RPC",
  "EXAMPLE_WEBSOCKET_URL",
  "EXAMPLE_API_KEY",
  "EXAMPLE_SECRET",
  "EXAMPLE_MNEMONIC",
  "EXAMPLE_AUTH_TOKEN",
];

describe("envVarLogging", () => {
  it("detects environment-variable context names", () => {
    expect(isEnvVarContext("EXAMPLE_CONTRACT_ADDRESS")).to.equal(true);
    expect(isEnvVarContext("EXAMPLE_OPERATORS[0]")).to.equal(true);
    expect(isEnvVarContext("address")).to.equal(false);
  });

  it("redacts values when the env var name matches the denylist", () => {
    for (const name of SENSITIVE_ENV_VAR_EXAMPLES) {
      expect(isSensitiveEnvVar(name), name).to.equal(true);
      expect(formatEnvVarForLog(name, "secret-value"), name).to.equal(name);
      expect(formatEnvVarValueForMessage(name, "secret-value"), name).to.equal("[REDACTED]");
    }
  });

  it("logs values for env var names outside the denylist", () => {
    const name = "EXAMPLE_DEPLOY_PARAM";
    expect(isSensitiveEnvVar(name)).to.equal(false);
    expect(formatEnvVarForLog(name, "example")).to.equal(`${name}=example`);
    expect(formatEnvVarValueForMessage(name, "example")).to.equal("example");
  });

  it("does not redact file-path env vars that are not credentials", () => {
    expect(isSensitiveEnvVar("ADDRESS_FILTER_FILE_PATH")).to.equal(false);
    expect(formatEnvVarForLog("ADDRESS_FILTER_FILE_PATH", "/tmp/filter.txt")).to.equal(
      "ADDRESS_FILTER_FILE_PATH=/tmp/filter.txt",
    );
  });
});
