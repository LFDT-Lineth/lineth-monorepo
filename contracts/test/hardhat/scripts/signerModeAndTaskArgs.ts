import { expect } from "chai";

import { resolveDeployerAccounts } from "../../../scripts/hardhat/deployer-accounts";
import {
  assertExclusiveSignerMode,
  getSignerModeConflictMessage,
  hasConfiguredDeployerPrivateKey,
  isSignerUiEnabled,
} from "../../../scripts/hardhat/signer-mode";
import { parseVerifyTaskArgs, stringifyVerifyTaskArgs } from "../../../scripts/hardhat/verify-task-args";

describe("signer mode and hardhat task-args helpers", () => {
  it("signer mode helpers detect configured env values", () => {
    expect(hasConfiguredDeployerPrivateKey(undefined)).to.equal(false);
    expect(hasConfiguredDeployerPrivateKey("   ")).to.equal(false);
    expect(hasConfiguredDeployerPrivateKey("0x1234")).to.equal(true);

    expect(isSignerUiEnabled(undefined)).to.equal(false);
    expect(isSignerUiEnabled("false")).to.equal(false);
    expect(isSignerUiEnabled("true")).to.equal(true);
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
        deployerPrivateKey: undefined,
      }),
    ).to.deep.equal([]);

    expect(
      resolveDeployerAccounts({
        hardhatSignerUiEnabled: undefined,
        deployerPrivateKey: undefined,
      }),
    ).to.deep.equal([]);

    expect(
      resolveDeployerAccounts({
        hardhatSignerUiEnabled: undefined,
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

  it("stringifies and parses verify task args including bigint values", () => {
    const input = {
      address: "0x000000000000000000000000000000000000dEaD",
      constructorArguments: [1n, { nested: 2n }],
      libraries: {
        Lib: "0x0000000000000000000000000000000000000001",
      },
    };

    const serialized = stringifyVerifyTaskArgs(input);
    const parsed = parseVerifyTaskArgs(serialized);

    expect(parsed.address).to.equal(input.address);
    const parsedArgs = parsed.constructorArguments as unknown[];
    expect(parsedArgs[0]).to.equal(1n);
    expect((parsedArgs[1] as { nested: bigint }).nested).to.equal(2n);
  });
});
