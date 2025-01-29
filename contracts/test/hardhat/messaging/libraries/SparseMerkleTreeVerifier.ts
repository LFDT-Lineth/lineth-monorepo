import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";
import { TestSparseMerkleTreeVerifier } from "../../../../typechain-types";
import { MAX_UINT32, MAX_UINT33, MESSAGE_FEE, MESSAGE_VALUE_1ETH } from "../../common/constants";
import { deployFromFactory } from "../../common/deployment";
import { expectRevertWithCustomError, generateRandomBytes, range } from "../../common/helpers";

describe("SparseMerkleTreeVerifier", () => {
  let sparseMerkleTreeVerifier: TestSparseMerkleTreeVerifier;
  let sender: SignerWithAddress;
  let recipient: SignerWithAddress;

  async function deploySparseMerkleTreeVerifierFixture() {
    return deployFromFactory("TestSparseMerkleTreeVerifier") as Promise<TestSparseMerkleTreeVerifier>;
  }

  beforeEach(async () => {
    [sender, recipient] = await ethers.getSigners();
    sparseMerkleTreeVerifier = await loadFixture(deploySparseMerkleTreeVerifierFixture);
  });

  describe("efficientKeccak", () => {
    const leftValue = "0xee0d96289fd184187cae377d3b9771b5d11632a67b53a0cf55d9f1283a1ff3df";
    const rightValue = "0xa49b884fb0575def8f20411b79cabc508d0fca402f124d4701a619516de1b8a5";

    it("Should return the same keccak hash as the solidity keccak precompile", async () => {
      const solidityKeccakHash = ethers.solidityPackedKeccak256(["bytes32", "bytes32"], [leftValue, rightValue]);
      const yulKeccakHash = await sparseMerkleTreeVerifier.efficientKeccak(leftValue, rightValue);

      expect(yulKeccakHash).to.equal(solidityKeccakHash);
    });

    it("Should return the correct keccak hash", async () => {
      const expectedHash = "0x85759ffc815fe5727bc6a99009655ad56bd03e3a24e4866fce1e6b2797d91320";
      expect(await sparseMerkleTreeVerifier.efficientKeccak(leftValue, rightValue)).to.equal(expectedHash);
    });
  });

  describe("verifyMerkleProof", () => {
    it("Should return false if the proof is invalid", async () => {
      const messageNumber = 1;
      const leafHash = await sparseMerkleTreeVerifier.getLeafHash(
        sender.address,
        recipient.address,
        MESSAGE_FEE,
        MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        messageNumber,
        "0x",
      );

      const merkleTreeRootHash = "0xe2f0c51f5fe51164a1712f5e6f1ee6b08bb2c2f2589c7c44fee8566a5cbbcf76";
      const leafIndex = 1;
      const proof = [
        "0xc7fbd6b64ad6e0bf300924171e95bfa232ecd8e0a2253e8ae341f9d9da9e28c7",
        "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
        "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
        "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
        "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
        "0x0eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d",
        "0x887c22bd8750d34016ac3c66b5ff102dacdd73f6b014e710b51e8022af9a1968",
        "0xffd70157e48063fc33c97a050f7f640233bf646cc98d9524c6b92bcf3ab56f83",
        "0x9867cc5f7f196b93bae1e27e6320742445d290f2263827498b54fec539f756af",
        "0xc1df82d9c4b87413eae2ef048f94b4d3554cea73d92b0f7af96e0271c691e2bb",
        "0xf9dc3e7fe016e050eff260334f18a5d4fe391d82092319f5964f2e2eb7c1c3a5",
        "0xf8b13a49e282f609c317a833fb8d976d11517c571d1221a265d25af778ecf892",
        "0x5a2dce0a8a7f68bb74560f8f71837c2c2ebbcbf7fffb42ae1896f13f7c7479a0",
        "0xc1df82d9c4b87413eae2ef048f94b4d3554cea73d92b0f7af96e0271c691e2bb",
        "0x5c67add7c6caf302256adedf7ab114da0acfe870d449a3a489f781d659e8becc",
        "0xda7bce9f4e8618b6bd2f4132ce798cdc7a60e7e1460a7299e3c6342a579626d2",
        "0x2733e50f526ec2fa19a22b31e8ed50f23cd1fdf94c9154ed3a7609a2f1ff981f",
        "0x5a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e377",
        "0x5a2dce0a8a7f68bb74560f8f71837c2c2ebbcbf7fffb42ae1896f13f7c7479a0",
        "0xb46a28b6f55540f89444f63de0378e3d121be09e06cc9ded1c20e65876d36aa0",
        "0xc65e9645644786b620e2dd2ad648ddfcbf4a7e5b1a3a4ecfe7f64667a3f0b7e2",
        "0xf4418588ed35a2458cffeb39b93d26f18d2ab13bdce6aee58e7b99359ec2dfd9",
        "0x5a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e377",
        "0x4df84f40ae0c8229d0d6069e5c8f39a7c299677a09d367fc7b05e3bc380ee652",
        "0xcdc72595f74c7b1043d0e1ffbab734648c838dfb0527d971b602bc216c9619ef",
        "0x0abf5ac974a1ed57f4050aa510dd9c74f508277b39d7973bb2dfccc5eeb0618d",
        "0xb8cd74046ff337f0a7bf2c8e03e10f642c1886798d71806ab1e888d9e5ee87d0",
        "0x838c5655cb21c6cb83313b5a631175dff4963772cce9108188b34ac87c81c41e",
        "0x662ee4dd2dd7b2bc707961b1e646c4047669dcb6584f0d8d770daf5d7e7deb2e",
        "0x388ab20e2573d171a88108e79d820e98f26c0b84aa8b2f4aa4968dbb818ea322",
        "0x93237c50ba75ee485f4c22adf2f741400bdf8d6a9cc7df7ecae576221665d735",
        "0x8448818bb4ae4562849e949e17ac16e0be16688e156b5cf15e098c627c0056a9",
      ];

      expect(await sparseMerkleTreeVerifier.verifyMerkleProof(leafHash, proof, leafIndex, merkleTreeRootHash)).to.be
        .false;
    });

    it("Should return true if the proof is valid", async () => {
      const messageNumber = 1;
      const leafHash = await sparseMerkleTreeVerifier.getLeafHash(
        sender.address,
        recipient.address,
        MESSAGE_FEE,
        MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        messageNumber,
        "0x",
      );

      const merkleTreeRootHash = "0xe2f0c51f5fe51164a1712f5e6f1ee6b08bb2c2f2589c7c44fee8566a5cbbcf76";
      const leafIndex = 1;
      const proof = [
        "0xc7fbd6b64ad6e0bf300924171e95bfa232ecd8e0a2253e8ae341f9d9da9e28c7",
        "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
        "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
        "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
        "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
        "0x0eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d",
        "0x887c22bd8750d34016ac3c66b5ff102dacdd73f6b014e710b51e8022af9a1968",
        "0xffd70157e48063fc33c97a050f7f640233bf646cc98d9524c6b92bcf3ab56f83",
        "0x9867cc5f7f196b93bae1e27e6320742445d290f2263827498b54fec539f756af",
        "0xcefad4e508c098b9a7e1d8feb19955fb02ba9675585078710969d3440f5054e0",
        "0xf9dc3e7fe016e050eff260334f18a5d4fe391d82092319f5964f2e2eb7c1c3a5",
        "0xf8b13a49e282f609c317a833fb8d976d11517c571d1221a265d25af778ecf892",
        "0x3490c6ceeb450aecdc82e28293031d10c7d73bf85e57bf041a97360aa2c5d99c",
        "0xc1df82d9c4b87413eae2ef048f94b4d3554cea73d92b0f7af96e0271c691e2bb",
        "0x5c67add7c6caf302256adedf7ab114da0acfe870d449a3a489f781d659e8becc",
        "0xda7bce9f4e8618b6bd2f4132ce798cdc7a60e7e1460a7299e3c6342a579626d2",
        "0x2733e50f526ec2fa19a22b31e8ed50f23cd1fdf94c9154ed3a7609a2f1ff981f",
        "0xe1d3b5c807b281e4683cc6d6315cf95b9ade8641defcb32372f1c126e398ef7a",
        "0x5a2dce0a8a7f68bb74560f8f71837c2c2ebbcbf7fffb42ae1896f13f7c7479a0",
        "0xb46a28b6f55540f89444f63de0378e3d121be09e06cc9ded1c20e65876d36aa0",
        "0xc65e9645644786b620e2dd2ad648ddfcbf4a7e5b1a3a4ecfe7f64667a3f0b7e2",
        "0xf4418588ed35a2458cffeb39b93d26f18d2ab13bdce6aee58e7b99359ec2dfd9",
        "0x5a9c16dc00d6ef18b7933a6f8dc65ccb55667138776f7dea101070dc8796e377",
        "0x4df84f40ae0c8229d0d6069e5c8f39a7c299677a09d367fc7b05e3bc380ee652",
        "0xcdc72595f74c7b1043d0e1ffbab734648c838dfb0527d971b602bc216c9619ef",
        "0x0abf5ac974a1ed57f4050aa510dd9c74f508277b39d7973bb2dfccc5eeb0618d",
        "0xb8cd74046ff337f0a7bf2c8e03e10f642c1886798d71806ab1e888d9e5ee87d0",
        "0x838c5655cb21c6cb83313b5a631175dff4963772cce9108188b34ac87c81c41e",
        "0x662ee4dd2dd7b2bc707961b1e646c4047669dcb6584f0d8d770daf5d7e7deb2e",
        "0x388ab20e2573d171a88108e79d820e98f26c0b84aa8b2f4aa4968dbb818ea322",
        "0x93237c50ba75ee485f4c22adf2f741400bdf8d6a9cc7df7ecae576221665d735",
        "0x8448818bb4ae4562849e949e17ac16e0be16688e156b5cf15e098c627c0056a9",
      ];

      expect(await sparseMerkleTreeVerifier.verifyMerkleProof(leafHash, proof, leafIndex, merkleTreeRootHash)).to.be
        .true;
    });

    // Test will fail if leafIndex > 2 ** 31 as it exceeds the uint32 max value.
    it("Should revert with LeafIndexOutOfBounds when leaf index is too large", async () => {
      const messageNumber = 1;
      const leafHash = await sparseMerkleTreeVerifier.getLeafHash(
        sender.address,
        recipient.address,
        MESSAGE_FEE,
        MESSAGE_FEE + MESSAGE_VALUE_1ETH,
        messageNumber,
        "0x",
      );

      const merkleTreeRootHash = "0xe2f0c51f5fe51164a1712f5e6f1ee6b08bb2c2f2589c7c44fee8566a5cbbcf76";
      const proof = [
        "0xc7fbd6b64ad6e0bf300924171e95bfa232ecd8e0a2253e8ae341f9d9da9e28c7",
        "0xad3228b676f7d3cd4284a5443f17f1962b36e491b30a40b2405849e597ba5fb5",
        "0xb4c11951957c6f8f642c4af61cd6b24640fec6dc7fc607ee8206a99e92410d30",
        "0x21ddb9a356815c3fac1026b6dec5df3124afbadb485c9ba5a3e3398a04b7ba85",
        "0xe58769b32a1beaf1ea27375a44095a0d1fb664ce2dd358e7fcbfb78c26a19344",
        "0x0eb01ebfc9ed27500cd4dfc979272d1f0913cc9f66540d7e8005811109e1cf2d",
        "0x887c22bd8750d34016ac3c66b5ff102dacdd73f6b014e710b51e8022af9a1968",
        "0xffd70157e48063fc33c97a050f7f640233bf646cc98d9524c6b92bcf3ab56f83",
        "0x9867cc5f7f196b93bae1e27e6320742445d290f2263827498b54fec539f756af",
        "0xcefad4e508c098b9a7e1d8feb19955fb02ba9675585078710969d3440f5054e0",
        "0xf9dc3e7fe016e050eff260334f18a5d4fe391d82092319f5964f2e2eb7c1c3a5",
      ];

      // Using a leaf index larger than 2 ** proof.length - 1
      const invalidLeafIndex = 2 ** proof.length;

      await expectRevertWithCustomError(
        sparseMerkleTreeVerifier,
        sparseMerkleTreeVerifier.verifyMerkleProof(leafHash, proof, invalidLeafIndex, merkleTreeRootHash),
        "LeafIndexOutOfBounds",
        [invalidLeafIndex, 2 ** proof.length - 1],
      );
    });

    it("Should revert if casting to a value higher than 32 bit uint", async () => {
      const testValue = MAX_UINT32 + 1n;
      await expectRevertWithCustomError(
        sparseMerkleTreeVerifier,
        sparseMerkleTreeVerifier.testSafeCastToUint32(testValue),
        "SafeCastOverflowedUintDowncast",
        [32, testValue],
      );
    });

    it("Should revert if proof length results in casting to a value higher than 32 bit uint", async () => {
      const proof = range(0, 32).map(() => generateRandomBytes(32));

      await expectRevertWithCustomError(
        sparseMerkleTreeVerifier,
        sparseMerkleTreeVerifier.verifyMerkleProof(generateRandomBytes(32), proof, 25, generateRandomBytes(32)),
        "SafeCastOverflowedUintDowncast",
        [32, MAX_UINT33],
      );
    });

    it("Should cast if casting to max uint 32", async () => {
      const testValue = MAX_UINT32;

      expect(await sparseMerkleTreeVerifier.testSafeCastToUint32(testValue)).to.equal(testValue);
    });

    it("Should cast if casting to lower than max uint 32", async () => {
      const testValue = MAX_UINT32 - 1n;

      expect(await sparseMerkleTreeVerifier.testSafeCastToUint32(testValue)).to.equal(testValue);
    });
  });
});
