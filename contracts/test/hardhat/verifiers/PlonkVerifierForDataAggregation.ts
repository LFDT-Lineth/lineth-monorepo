import { expect } from "chai";
import { toBeHex } from "ethers";
import { ethers } from "hardhat";

import { IPlonkVerifier, PlonkVerifierForDataAggregation__factory } from "../../../typechain-types";
import { expectEventDirectFromReceiptData, expectRevertWithCustomError } from "../common/helpers";

describe("PlonkVerifierForDataAggregation", () => {
  async function deployContract(params: IPlonkVerifier.ChainConfigurationParameterStruct[]) {
    const factory = await ethers.getContractFactory("PlonkVerifierForDataAggregation");
    const verifier = await factory.deploy(params);
    await verifier.waitForDeployment();
    return verifier;
  }

  describe("Deployment", () => {
    it("Should revert when no chain configuration has been provided", async () => {
      await expectRevertWithCustomError(
        new PlonkVerifierForDataAggregation__factory(),
        deployContract([]),
        "ChainConfigurationNotProvided",
      );
    });

    it("Should deploy with one configuration value that has a first 0 bit", async () => {
      const chainId = toBeHex(1337, 32);

      const params = [
        {
          value: chainId,
          name: "chainId",
        },
      ];
      const verifier = await deployContract(params);
      const receipt = await verifier.deploymentTransaction()?.wait();

      const expectedConfigurationHash = ethers.keccak256(chainId);

      expectEventDirectFromReceiptData(verifier, receipt!, "ChainConfigurationSet", [
        expectedConfigurationHash,
        [[chainId, "chainId"]],
      ]);
    });

    it("Should deploy with one configuration value that has a first non 0 bit", async () => {
      const chainId = toBeHex("0x8900000000000000000000000000000000000000000000000000000000000089", 32);

      const params = [
        {
          value: chainId,
          name: "chainId",
        },
      ];
      const verifier = await deployContract(params);
      const receipt = await verifier.deploymentTransaction()?.wait();

      const leastSignificantBit = BigInt(chainId) >> 128n;
      const mostSignificantBit = BigInt(chainId) & BigInt("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF");

      const hashPayload = ethers.concat([toBeHex(leastSignificantBit, 32), toBeHex(mostSignificantBit, 32)]);

      const expectedConfigurationHash = ethers.keccak256(hashPayload);

      expectEventDirectFromReceiptData(verifier, receipt!, "ChainConfigurationSet", [
        expectedConfigurationHash,
        [[chainId, "chainId"]],
      ]);
    });

    it("Should deploy with multiple configuration values that have a first 0 bit", async () => {
      const chainId = toBeHex(1337, 32);
      const baseFee = toBeHex(7, 32);
      const l2MessageServiceAddress = toBeHex("0xe537D669CA013d86EBeF1D64e40fC74CADC91987", 32);

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

      const verifier = await deployContract(params);
      const receipt = await verifier.deploymentTransaction()?.wait();

      const hashPayload = ethers.concat([chainId, baseFee, l2MessageServiceAddress]);
      const expectedConfigurationHash = ethers.keccak256(hashPayload);

      expectEventDirectFromReceiptData(verifier, receipt!, "ChainConfigurationSet", [
        expectedConfigurationHash,
        [
          [chainId, "chainId"],
          [baseFee, "baseFee"],
          [l2MessageServiceAddress, "l2MessageServiceAddress"],
        ],
      ]);
    });

    it("Should deploy with multiple configuration values that have a first non 0 bit", async () => {
      const chainId = toBeHex("0x8900000000000000000000000000000000000000000000000000000000000089", 32);
      const baseFee = toBeHex(7, 32);
      const l2MessageServiceAddress = toBeHex("0xe537D669CA013d86EBeF1D64e40fC74CADC91987", 32);

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

      const verifier = await deployContract(params);
      const receipt = await verifier.deploymentTransaction()?.wait();

      const leastSignificantBit = BigInt(chainId) >> 128n;
      const mostSignificantBit = BigInt(chainId) & BigInt("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF");

      const hashPayload = ethers.concat([
        toBeHex(leastSignificantBit, 32),
        toBeHex(mostSignificantBit, 32),
        baseFee,
        l2MessageServiceAddress,
      ]);

      const expectedConfigurationHash = ethers.keccak256(hashPayload);

      expectEventDirectFromReceiptData(verifier, receipt!, "ChainConfigurationSet", [
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
      const chainId = toBeHex(1337, 32);
      const baseFee = toBeHex(7, 32);
      const l2MessageServiceAddress = toBeHex("0xe537D669CA013d86EBeF1D64e40fC74CADC91987", 32);

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

      const verifier = await deployContract(params);

      const hashPayload = ethers.concat([chainId, baseFee, l2MessageServiceAddress]);
      const expectedConfigurationHash = ethers.keccak256(hashPayload);

      expect(await verifier.getChainConfiguration()).to.be.equal(expectedConfigurationHash);
    });
  });
});
