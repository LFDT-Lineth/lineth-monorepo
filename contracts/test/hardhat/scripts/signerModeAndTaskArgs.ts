import { expect } from "chai";
import { describe, it } from "node:test";

import { resolveDeployerAccounts } from "../../../hardhat-plugins/signer-ui/signer-mode.js";
import {
  assertExclusiveSignerMode,
  getSignerModeConflictMessage,
  hasConfiguredDeployerPrivateKey,
  isSignerUiEnvSet,
} from "../../../hardhat-plugins/signer-ui/signer-mode.js";

describe("signer mode and hardhat task-args helpers", () => {
  it("signer mode helpers detect configured env values", () => {
    expect(hasConfiguredDeployerPrivateKey(undefined)).to.equal(false);
    expect(hasConfiguredDeployerPrivateKey("   ")).to.equal(false);
    expect(hasConfiguredDeployerPrivateKey("0x1234")).to.equal(true);

    expect(isSignerUiEnvSet(undefined)).to.equal(false);
    expect(isSignerUiEnvSet("false")).to.equal(false);
    expect(isSignerUiEnvSet("true")).to.equal(true);
  });

  it("throws on exclusive signer-mode conflict", () => {
    expect(() =>
      assertExclusiveSignerMode({
        hardhatSignerUiEnabled: true,
        deployerPrivateKeyConfigured: true,
      }),
    ).to.throw(getSignerModeConflictMessage());
  });

  it("resolves deployer accounts with expected backward-compatible behavior", () => {
    expect(
      resolveDeployerAccounts({
        hardhatSignerUiEnabled: "true",
      }),
    ).to.deep.equal([]);

    expect(resolveDeployerAccounts({})).to.deep.equal([]);

    expect(
      resolveDeployerAccounts({
        deployerPrivateKey: "1",
      }),
    ).to.deep.equal(["0x1"]);
  });

  it("rejects invalid or zero deployer private key values", () => {
    expect(() =>
      resolveDeployerAccounts({
        deployerPrivateKey: "not-hex",
      }),
    ).to.throw("DEPLOYER_PRIVATE_KEY is not valid hex.");

    expect(() =>
      resolveDeployerAccounts({
        deployerPrivateKey: "0x0",
      }),
    ).to.throw("DEPLOYER_PRIVATE_KEY cannot be zero.");
  });
});
