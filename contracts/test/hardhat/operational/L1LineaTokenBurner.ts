import { ethers, network } from "hardhat";
import { expect } from "chai";
import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { deployFromFactory, deployUpgradableFromFactory } from "../common/deployment";
import { L1LineaTokenBurner, MockL1LineaToken, TestL1MessageServiceMerkleProof } from "../../../typechain-types";
import { expectEvent, expectRevertWithCustomError } from "../common/helpers";
import {
  ADDRESS_ZERO,
  EMPTY_CALLDATA,
  INITIAL_WITHDRAW_LIMIT,
  MESSAGE_VALUE_1ETH,
  ONE_DAY_IN_SECONDS,
  pauseTypeRoles,
  unpauseTypeRoles,
  VALID_MERKLE_PROOF_WITH_ZERO_FEE,
} from "../common/constants";

// TODO: Dynamically generate a valid merkle proof for testing instead of using a hardcoded one.
// Should trigger a call to the token bridge to mint tokens on L1 to the burner address as part of the test setup.
describe("L1LineaTokenBurner", () => {
  let l1LineaTokenBurner: L1LineaTokenBurner;
  let admin: SignerWithAddress;
  let lineaToken: MockL1LineaToken;
  let messageService: TestL1MessageServiceMerkleProof;

  const INITIAL_LINEA_TOKEN_SUPPLY = ethers.parseUnits("100", 18);

  async function deployTestL1MessageServiceFixture(): Promise<TestL1MessageServiceMerkleProof> {
    return deployUpgradableFromFactory(
      "TestL1MessageServiceMerkleProof",
      [ONE_DAY_IN_SECONDS, INITIAL_WITHDRAW_LIMIT, pauseTypeRoles, unpauseTypeRoles],
      { unsafeAllow: ["constructor", "incorrect-initializer-order"] },
    ) as unknown as Promise<TestL1MessageServiceMerkleProof>;
  }

  async function deployL1LineaTokenBurnerContractFixture() {
    const messageService = await loadFixture(deployTestL1MessageServiceFixture);

    const l2LineaToken = await deployFromFactory("TestERC20", "TestERC20", "TEST", ethers.parseUnits("1000000000", 18));
    const lineaTokenFn = async () =>
      deployUpgradableFromFactory(
        "MockL1LineaToken",
        [await messageService.getAddress(), await l2LineaToken.getAddress(), "TestERC20", "TEST"],
        { unsafeAllow: ["constructor", "incorrect-initializer-order"] },
      ) as unknown as Promise<MockL1LineaToken>;

    const l1LineaToken = (await loadFixture(lineaTokenFn)) as MockL1LineaToken;

    const l1LineaTokenBurnerFn = async () =>
      deployFromFactory("L1LineaTokenBurner", await messageService.getAddress(), await l1LineaToken.getAddress());
    const l1LineaTokenBurner = (await loadFixture(l1LineaTokenBurnerFn)) as L1LineaTokenBurner;

    return { l1LineaTokenBurner, lineaToken: l1LineaToken, messageService };
  }

  before(async () => {
    await network.provider.send("hardhat_reset");
    [admin] = await ethers.getSigners();
  });

  beforeEach(async () => {
    ({ l1LineaTokenBurner, lineaToken, messageService } = await loadFixture(deployL1LineaTokenBurnerContractFixture));
    // Mint some LINEA tokens to the admin so that we can test the burning functionality.
    // It is used to initialize the total supply of the token.
    await lineaToken.mint(admin.address, INITIAL_LINEA_TOKEN_SUPPLY);
  });

  describe("construtor", () => {
    it("Should revert when messageService address is zero", async () => {
      await expectRevertWithCustomError(
        l1LineaTokenBurner,
        deployFromFactory("L1LineaTokenBurner", ADDRESS_ZERO, await lineaToken.getAddress()),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should revert when LINEA token address is zero", async () => {
      await expectRevertWithCustomError(
        l1LineaTokenBurner,
        deployFromFactory("L1LineaTokenBurner", await messageService.getAddress(), ADDRESS_ZERO),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should emit an event when initialized", async () => {
      const contract = await deployFromFactory(
        "L1LineaTokenBurner",
        await messageService.getAddress(),
        await lineaToken.getAddress(),
      );

      const receipt = await contract.deploymentTransaction()?.wait();
      const logs = receipt?.logs;

      expect(logs).to.have.lengthOf(1);

      const event = l1LineaTokenBurner.interface.parseLog(logs![0]);
      expect(event).is.not.null;
      expect(event!.name).to.equal("L1LineaTokenBurnerInitialized");
      expect(event!.args.messageService).to.equal(await messageService.getAddress());
      expect(event!.args.lineaToken).to.equal(await lineaToken.getAddress());
    });

    it("Should set the correct addresses", async () => {
      const lineaTokenAddress = await l1LineaTokenBurner.LINEA_TOKEN();
      const messageServiceAddress = await l1LineaTokenBurner.MESSAGE_SERVICE();

      expect(lineaTokenAddress).to.equal(await lineaToken.getAddress());
      expect(messageServiceAddress).to.equal(await messageService.getAddress());
    });
  });

  describe("claimMessageWithProof", () => {
    it("Should revert when message is not anchored on L1", async () => {
      await expectRevertWithCustomError(
        messageService,
        l1LineaTokenBurner.claimMessageWithProof({
          proof: VALID_MERKLE_PROOF_WITH_ZERO_FEE.proof,
          messageNumber: 1,
          leafIndex: VALID_MERKLE_PROOF_WITH_ZERO_FEE.index,
          from: admin.address,
          to: admin.address,
          fee: 0n,
          value: MESSAGE_VALUE_1ETH,
          feeRecipient: ADDRESS_ZERO,
          merkleRoot: VALID_MERKLE_PROOF_WITH_ZERO_FEE.merkleRoot,
          data: EMPTY_CALLDATA,
        }),
        "L2MerkleRootDoesNotExist",
      );
    });

    it("Should revert when burner contract balance == 0", async () => {
      await messageService.setL2L1MessageToClaimed(1);

      const burnerBalanceBefore = await lineaToken.balanceOf(await l1LineaTokenBurner.getAddress());
      expect(burnerBalanceBefore).to.equal(0);

      const txPromise = l1LineaTokenBurner.claimMessageWithProof({
        proof: VALID_MERKLE_PROOF_WITH_ZERO_FEE.proof,
        messageNumber: 1,
        leafIndex: VALID_MERKLE_PROOF_WITH_ZERO_FEE.index,
        from: admin.address,
        to: admin.address,
        fee: 0n,
        value: MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: VALID_MERKLE_PROOF_WITH_ZERO_FEE.merkleRoot,
        data: EMPTY_CALLDATA,
      });
      await expectRevertWithCustomError(l1LineaTokenBurner, txPromise, "NoTokensToBurn");
    });

    it("Should only burn LINEA tokens and syncTotalSupplyToL2 when message is already claimed and burner contract balance > 0", async () => {
      await messageService.setL2L1MessageToClaimed(1);

      const lineaTokensBalanceOwnedByBurnerContract = ethers.parseUnits("100", 18);
      await lineaToken.mint(await l1LineaTokenBurner.getAddress(), lineaTokensBalanceOwnedByBurnerContract);

      const totalSupplyBeforeBurning = await lineaToken.totalSupply();
      expect(totalSupplyBeforeBurning).to.equal(INITIAL_LINEA_TOKEN_SUPPLY + lineaTokensBalanceOwnedByBurnerContract);

      const burnerBalanceBefore = await lineaToken.balanceOf(await l1LineaTokenBurner.getAddress());
      expect(burnerBalanceBefore).to.equal(lineaTokensBalanceOwnedByBurnerContract);

      const txPromise = l1LineaTokenBurner.claimMessageWithProof({
        proof: VALID_MERKLE_PROOF_WITH_ZERO_FEE.proof,
        messageNumber: 1,
        leafIndex: VALID_MERKLE_PROOF_WITH_ZERO_FEE.index,
        from: admin.address,
        to: admin.address,
        fee: 0n,
        value: MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: VALID_MERKLE_PROOF_WITH_ZERO_FEE.merkleRoot,
        data: EMPTY_CALLDATA,
      });

      await expectEvent(lineaToken, txPromise, "Transfer", [
        await l1LineaTokenBurner.getAddress(),
        ADDRESS_ZERO,
        lineaTokensBalanceOwnedByBurnerContract,
      ]);

      const burnerBalanceAfter = await lineaToken.balanceOf(await l1LineaTokenBurner.getAddress());
      expect(burnerBalanceAfter).to.equal(0);
    });

    it("Should claim message, burn LINEA tokens and syncTotalSupplyToL2 when message is not yet claimed and burner contract balance > 0", async () => {
      await messageService.addFunds({ value: INITIAL_WITHDRAW_LIMIT * 2n });

      const lineaTokensBalanceOwnedByBurnerContract = ethers.parseUnits("100", 18);
      await lineaToken.mint(await l1LineaTokenBurner.getAddress(), lineaTokensBalanceOwnedByBurnerContract);

      const burnerBalanceBefore = await lineaToken.balanceOf(await l1LineaTokenBurner.getAddress());
      expect(burnerBalanceBefore).to.equal(lineaTokensBalanceOwnedByBurnerContract);

      await messageService.addL2MerkleRoots(
        [VALID_MERKLE_PROOF_WITH_ZERO_FEE.merkleRoot],
        VALID_MERKLE_PROOF_WITH_ZERO_FEE.proof.length,
      );

      const txPromise = l1LineaTokenBurner.claimMessageWithProof({
        proof: VALID_MERKLE_PROOF_WITH_ZERO_FEE.proof,
        messageNumber: 1,
        leafIndex: VALID_MERKLE_PROOF_WITH_ZERO_FEE.index,
        from: admin.address,
        to: admin.address,
        fee: 0n,
        value: MESSAGE_VALUE_1ETH,
        feeRecipient: ADDRESS_ZERO,
        merkleRoot: VALID_MERKLE_PROOF_WITH_ZERO_FEE.merkleRoot,
        data: EMPTY_CALLDATA,
      });

      const messageLeafHash = ethers.keccak256(
        ethers.AbiCoder.defaultAbiCoder().encode(
          ["address", "address", "uint256", "uint256", "uint256", "bytes"],
          [admin.address, admin.address, 0n, MESSAGE_VALUE_1ETH, "1", EMPTY_CALLDATA],
        ),
      );

      await expectEvent(messageService, txPromise, "MessageClaimed", [messageLeafHash]);
      await expectEvent(lineaToken, txPromise, "Transfer", [
        await l1LineaTokenBurner.getAddress(),
        ADDRESS_ZERO,
        lineaTokensBalanceOwnedByBurnerContract,
      ]);
    });
  });
});
