import { expect } from "chai";
import hre, { network } from "hardhat";
import { describe, it, before } from "node:test";
import { type Hex, concat, toHex } from "viem";

import { deployFromFactory } from "../common/deployment.js";
import { expectEventDirectFromReceiptData, expectRevertWithCustomError } from "../common/helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("PlonkVerifierForDataAggregation", () => {
  let mimc: ContractReturnType<"Mimc">;

  /** Deploys the verifier and returns it with its deployment receipt. */
  async function deployContract(params: { value: Hex; name: string }[]) {
    const { viem } = await network.getOrCreate();
    const { contract: verifier, deploymentTransaction } = await viem.sendDeploymentTransaction(
      "PlonkVerifierForDataAggregation",
      [params],
      { libraries: { Mimc: mimc.address } },
    );
    const publicClient = await viem.getPublicClient();
    const receipt = await publicClient.waitForTransactionReceipt({ hash: deploymentTransaction.hash });
    return { verifier, receipt };
  }

  before(async () => {
    mimc = await deployFromFactory("Mimc");
  });

  describe("Deployment", () => {
    it("Should revert when no chain configuration has been provided", async () => {
      await expectRevertWithCustomError(
        await hre.artifacts.readArtifact("PlonkVerifierForDataAggregation"),
        deployContract([]),
        "ChainConfigurationNotProvided",
      );
    });

    it("Should deploy with one configuration value that has a first 0 bit", async () => {
      const chainId = toHex(1337, { size: 32 });

      const params = [
        {
          value: chainId,
          name: "chainId",
        },
      ];
      const { verifier, receipt } = await deployContract(params);

      const expectedConfigurationHash = await mimc.read.hash([chainId]);

      expectEventDirectFromReceiptData(verifier, receipt, "ChainConfigurationSet", [
        expectedConfigurationHash,
        [[chainId, "chainId"]],
      ]);
    });

    it("Should deploy with one configuration value that has a first non 0 bit", async () => {
      const chainId = toHex(BigInt("0x8900000000000000000000000000000000000000000000000000000000000089"), { size: 32 });

      const params = [
        {
          value: chainId,
          name: "chainId",
        },
      ];
      const { verifier, receipt } = await deployContract(params);

      const leastSignificantBit = BigInt(chainId) >> 128n;
      const mostSignificantBit = BigInt(chainId) & BigInt("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF");

      const mimcPayload = concat([toHex(leastSignificantBit, { size: 32 }), toHex(mostSignificantBit, { size: 32 })]);

      const expectedConfigurationHash = await mimc.read.hash([mimcPayload]);

      expectEventDirectFromReceiptData(verifier, receipt, "ChainConfigurationSet", [
        expectedConfigurationHash,
        [[chainId, "chainId"]],
      ]);
    });

    it("Should deploy with multiple configuration values that have a first 0 bit", async () => {
      const chainId = toHex(1337, { size: 32 });
      const baseFee = toHex(7, { size: 32 });
      const l2MessageServiceAddress = toHex(BigInt("0xe537D669CA013d86EBeF1D64e40fC74CADC91987"), { size: 32 });

      const params = [
        {
          value: chainId,
          name: "chainId",
        },
        {
          value: baseFee,
          name: "baseFee",
        },
        {
          value: l2MessageServiceAddress,
          name: "l2MessageServiceAddress",
        },
      ];

      const { verifier, receipt } = await deployContract(params);

      const mimcPayload = concat([chainId, baseFee, l2MessageServiceAddress]);
      const expectedConfigurationHash = await mimc.read.hash([mimcPayload]);

      expectEventDirectFromReceiptData(verifier, receipt, "ChainConfigurationSet", [
        expectedConfigurationHash,
        [
          [chainId, "chainId"],
          [baseFee, "baseFee"],
          [l2MessageServiceAddress, "l2MessageServiceAddress"],
        ],
      ]);
    });

    it("Should deploy with multiple configuration values that have a first non 0 bit", async () => {
      const chainId = toHex(BigInt("0x8900000000000000000000000000000000000000000000000000000000000089"), { size: 32 });
      const baseFee = toHex(7, { size: 32 });
      const l2MessageServiceAddress = toHex(BigInt("0xe537D669CA013d86EBeF1D64e40fC74CADC91987"), { size: 32 });

      const params = [
        {
          value: chainId,
          name: "chainId",
        },
        {
          value: baseFee,
          name: "baseFee",
        },
        {
          value: l2MessageServiceAddress,
          name: "l2MessageServiceAddress",
        },
      ];

      const { verifier, receipt } = await deployContract(params);

      const leastSignificantBit = BigInt(chainId) >> 128n;
      const mostSignificantBit = BigInt(chainId) & BigInt("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF");

      const mimcPayload = concat([
        toHex(leastSignificantBit, { size: 32 }),
        toHex(mostSignificantBit, { size: 32 }),
        baseFee,
        l2MessageServiceAddress,
      ]);

      const expectedConfigurationHash = await mimc.read.hash([mimcPayload]);

      expectEventDirectFromReceiptData(verifier, receipt, "ChainConfigurationSet", [
        expectedConfigurationHash,
        [
          [chainId, "chainId"],
          [baseFee, "baseFee"],
          [l2MessageServiceAddress, "l2MessageServiceAddress"],
        ],
      ]);
    });
  });

  describe("getChainConfiguration", () => {
    it("Should return the chain configuration hash", async () => {
      const chainId = toHex(1337, { size: 32 });
      const baseFee = toHex(7, { size: 32 });
      const l2MessageServiceAddress = toHex(BigInt("0xe537D669CA013d86EBeF1D64e40fC74CADC91987"), { size: 32 });

      const params = [
        {
          value: chainId,
          name: "chainId",
        },
        {
          value: baseFee,
          name: "baseFee",
        },
        {
          value: l2MessageServiceAddress,
          name: "l2MessageServiceAddress",
        },
      ];

      const { verifier } = await deployContract(params);

      const mimcPayload = concat([chainId, baseFee, l2MessageServiceAddress]);
      const expectedConfigurationHash = await mimc.read.hash([mimcPayload]);

      expect(await verifier.read.getChainConfiguration()).to.be.equal(expectedConfigurationHash);
    });
  });
});
