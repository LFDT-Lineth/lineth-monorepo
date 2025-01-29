import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture, time } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";
import {
  ADDRESS_ZERO,
  DEFAULT_ADMIN_ROLE,
  MINTER_ROLE,
  ONE_DAY_IN_SECONDS,
  RATE_LIMIT_SETTER_ROLE,
} from "../common/constants";
import { deployFromFactory } from "../common/deployment";
import { buildAccessErrorMessage, expectRevertWithCustomError, expectRevertWithReason } from "../common/helpers";
import { LineaVoyageXP, TokenMintingRateLimiter } from "../../../typechain-types";

describe("Token Minting Rate Limiter", () => {
  let tokenMintingRateLimiter: TokenMintingRateLimiter;
  let minter: SignerWithAddress;
  let deployer: SignerWithAddress;
  let defaultAdmin: SignerWithAddress;

  const mintingPeriodInSeconds: bigint = 5n;
  const mintingLimit: bigint = 10000n;
  const mintAmount: bigint = 1000n;

  let xpToken: LineaVoyageXP;
  let xpTokenAddress: string;

  let minterAddress: string;
  let deployerAddress: string;
  let defaultAdminAddress: string;

  async function deployLineaVoyageXPFixture() {
    return deployFromFactory("LineaVoyageXP", minterAddress) as Promise<LineaVoyageXP>;
  }

  async function deployTokenMintingRateLimiterFixture() {
    xpTokenAddress = await xpToken.getAddress();
    return deployFromFactory(
      "TokenMintingRateLimiter",
      xpTokenAddress,
      mintingPeriodInSeconds,
      mintingLimit,
      defaultAdminAddress,
      minterAddress,
    ) as Promise<TokenMintingRateLimiter>;
  }

  before(async () => {
    [deployer, minter, defaultAdmin] = await ethers.getSigners();
    minterAddress = await minter.getAddress();
    defaultAdminAddress = await defaultAdmin.getAddress();
    deployerAddress = await deployer.getAddress();
  });

  beforeEach(async () => {
    xpToken = await loadFixture(deployLineaVoyageXPFixture);
    tokenMintingRateLimiter = await loadFixture(deployTokenMintingRateLimiterFixture);
    await xpToken.connect(minter).grantRole(MINTER_ROLE, await tokenMintingRateLimiter.getAddress());
  });

  describe("Initialization and roles", () => {
    it("minter should have the 'MINTER_ROLE' role", async () => {
      expect(await tokenMintingRateLimiter.hasRole(MINTER_ROLE, minterAddress)).true;
    });

    it("default admin address should have the 'DEFAULT_ADMIN_ROLE' role", async () => {
      expect(await tokenMintingRateLimiter.hasRole(DEFAULT_ADMIN_ROLE, defaultAdminAddress)).true;
    });

    it("default admin address should have the 'RATE_LIMIT_SETTER_ROLE' role", async () => {
      expect(await tokenMintingRateLimiter.hasRole(RATE_LIMIT_SETTER_ROLE, defaultAdminAddress)).true;
    });

    it("deployer should NOT have the 'MINTER_ROLE' role", async () => {
      expect(await tokenMintingRateLimiter.hasRole(MINTER_ROLE, deployerAddress)).false;
    });

    it("deployer should NOT have the 'DEFAULT_ADMIN_ROLE' role", async () => {
      expect(await tokenMintingRateLimiter.hasRole(DEFAULT_ADMIN_ROLE, deployerAddress)).false;
    });

    it("fails to deploy when the token address is address zero", async () => {
      const deployCall = deployFromFactory(
        "TokenMintingRateLimiter",
        ADDRESS_ZERO,
        mintingPeriodInSeconds,
        mintingLimit,
        defaultAdminAddress,
        minterAddress,
      );

      await expectRevertWithCustomError(tokenMintingRateLimiter, deployCall, "ZeroAddressNotAllowed");
    });

    it("fails to deploy when mintingPeriodInSeconds is zero", async () => {
      const deployCall = deployFromFactory(
        "TokenMintingRateLimiter",
        xpTokenAddress,
        0n,
        mintingLimit,
        defaultAdminAddress,
        minterAddress,
      );

      await expectRevertWithCustomError(tokenMintingRateLimiter, deployCall, "PeriodIsZero");
    });

    it("fails to deploy when mintingLimit is zero", async () => {
      const deployCall = deployFromFactory(
        "TokenMintingRateLimiter",
        xpTokenAddress,
        mintingPeriodInSeconds,
        0n,
        defaultAdminAddress,
        minterAddress,
      );

      await expectRevertWithCustomError(tokenMintingRateLimiter, deployCall, "LimitIsZero");
    });

    it("fails to deploy when the default admin address is address zero", async () => {
      const deployCall = deployFromFactory(
        "TokenMintingRateLimiter",
        xpTokenAddress,
        mintingPeriodInSeconds,
        mintingLimit,
        ADDRESS_ZERO,
        minterAddress,
      );

      await expectRevertWithCustomError(tokenMintingRateLimiter, deployCall, "ZeroAddressNotAllowed");
    });

    it("fails to deploy when the minter address is address zero", async () => {
      const deployCall = deployFromFactory(
        "TokenMintingRateLimiter",
        xpTokenAddress,
        mintingPeriodInSeconds,
        mintingLimit,
        defaultAdminAddress,
        ADDRESS_ZERO,
      );

      await expectRevertWithCustomError(tokenMintingRateLimiter, deployCall, "ZeroAddressNotAllowed");
    });
  });

  describe("Single minting", () => {
    it("non-minter cannot mint tokens", async () => {
      await expectRevertWithReason(
        tokenMintingRateLimiter.connect(deployer).mint(deployerAddress, mintAmount),
        buildAccessErrorMessage(deployer, MINTER_ROLE),
      );
    });

    it("minter can mint tokens", async () => {
      expect(await tokenMintingRateLimiter.hasRole(MINTER_ROLE, minterAddress)).true;

      await tokenMintingRateLimiter.connect(minter).mint(deployerAddress, mintAmount);

      expect(await xpToken.balanceOf(deployerAddress)).to.be.equal(mintAmount);
    });
  });

  describe("Batch minting with one amount", () => {
    it("non-minter cannot mint tokens", async () => {
      await expectRevertWithReason(
        tokenMintingRateLimiter.batchMint([deployerAddress], mintAmount),
        buildAccessErrorMessage(deployer, MINTER_ROLE),
      );
    });

    it("minter can mint tokens for one address", async () => {
      await tokenMintingRateLimiter.connect(minter).batchMint([deployerAddress], mintAmount);

      expect(await xpToken.balanceOf(deployerAddress)).to.be.equal(mintAmount);
    });

    it("minter can mint tokens for multiple address", async () => {
      await tokenMintingRateLimiter.connect(minter).batchMint([minterAddress, deployerAddress], mintAmount);

      expect(await xpToken.balanceOf(deployerAddress)).to.be.equal(mintAmount);
      expect(await xpToken.balanceOf(minterAddress)).to.be.equal(mintAmount);
    });
  });

  describe("Batch minting with varying amounts", () => {
    it("non-minter cannot mint tokens", async () => {
      await expectRevertWithReason(
        tokenMintingRateLimiter.batchMintMultiple([deployerAddress], [1000n]),
        buildAccessErrorMessage(deployer, MINTER_ROLE),
      );
    });

    it("minter can mint tokens for one address", async () => {
      await tokenMintingRateLimiter.connect(minter).batchMintMultiple([deployerAddress], [1000n]);

      expect(await xpToken.balanceOf(deployerAddress)).to.be.equal(1000n);
    });

    it("minter can mint tokens for multiple address with different amounts", async () => {
      await tokenMintingRateLimiter.connect(minter).batchMintMultiple([minterAddress, deployerAddress], [1000n, 2000n]);

      expect(await xpToken.balanceOf(deployerAddress)).to.be.equal(2000n);
      expect(await xpToken.balanceOf(minterAddress)).to.be.equal(1000n);
    });

    it("cannot mint when array lengths are different", async () => {
      await expectRevertWithCustomError(
        tokenMintingRateLimiter,
        tokenMintingRateLimiter.connect(minter).batchMintMultiple([minterAddress, deployerAddress], [1000n]),
        "ArrayLengthsDoNotMatch",
      );
    });
  });

  describe("Rate limit values", () => {
    it("mintedAmountInPeriod increases when amounts withdrawn", async () => {
      expect(await tokenMintingRateLimiter.mintedAmountInPeriod()).to.be.equal(0);

      await tokenMintingRateLimiter.connect(minter).batchMint([deployerAddress], mintAmount);
      expect(await tokenMintingRateLimiter.mintedAmountInPeriod()).to.be.equal(mintAmount);
    });

    it("mintedAmountInPeriod increases to the limit", async () => {
      await tokenMintingRateLimiter.connect(minter).batchMint([deployerAddress], mintAmount);
      await tokenMintingRateLimiter.connect(minter).batchMint([deployerAddress], mintAmount * 9n);
      expect(await tokenMintingRateLimiter.mintedAmountInPeriod()).to.be.equal(mintingLimit);
    });

    it("withdrawing beyond the limit fails", async () => {
      await tokenMintingRateLimiter.connect(minter).batchMint([deployerAddress], mintingLimit);

      await expectRevertWithCustomError(
        tokenMintingRateLimiter,
        tokenMintingRateLimiter.connect(minter).mint(deployerAddress, 1),
        "RateLimitExceeded",
      );
    });

    it("used amount resets changing limit and time expired without changing values", async () => {
      await tokenMintingRateLimiter.connect(minter).batchMint([deployerAddress], mintingLimit);

      await expect(tokenMintingRateLimiter.connect(defaultAdmin).resetRateLimitAmount(mintAmount))
        .to.emit(tokenMintingRateLimiter, "LimitAmountChanged")
        .withArgs(defaultAdmin, mintAmount, true, false);

      expect(await tokenMintingRateLimiter.mintedAmountInPeriod()).to.be.equal(mintAmount);
      expect(await tokenMintingRateLimiter.mintingLimit()).to.be.equal(mintAmount);
    });

    it("used amount remains the same when increasing the limit", async () => {
      await tokenMintingRateLimiter.connect(minter).batchMint([deployerAddress], mintingLimit);

      await expect(tokenMintingRateLimiter.connect(defaultAdmin).resetRateLimitAmount(mintingLimit * 2n))
        .to.emit(tokenMintingRateLimiter, "LimitAmountChanged")
        .withArgs(defaultAdmin, mintingLimit * 2n, false, false);

      expect(await tokenMintingRateLimiter.mintedAmountInPeriod()).to.be.equal(mintingLimit);
      expect(await tokenMintingRateLimiter.mintingLimit()).to.be.equal(mintingLimit * 2n);
    });

    it("fails when trying to change limit with non-admin account", async () => {
      await expectRevertWithReason(
        tokenMintingRateLimiter.connect(minter).resetRateLimitAmount(mintingLimit * 2n),
        buildAccessErrorMessage(minter, RATE_LIMIT_SETTER_ROLE),
      );
    });

    it("used amount resets with time", async () => {
      await tokenMintingRateLimiter.connect(minter).batchMint([deployerAddress], mintingLimit);
      expect(await tokenMintingRateLimiter.connect(minter).mintedAmountInPeriod()).to.be.equal(mintingLimit);
      await time.increase(ONE_DAY_IN_SECONDS);
      await tokenMintingRateLimiter.connect(minter).mint(deployerAddress, 1);
      expect(await tokenMintingRateLimiter.mintedAmountInPeriod()).to.be.equal(1);
    });

    it("used amount resets when changing limit and time expired", async () => {
      await tokenMintingRateLimiter.connect(minter).batchMint([deployerAddress], mintingLimit);
      await time.increase(ONE_DAY_IN_SECONDS);

      await expect(tokenMintingRateLimiter.connect(defaultAdmin).resetRateLimitAmount(mintAmount))
        .to.emit(tokenMintingRateLimiter, "LimitAmountChanged")
        .withArgs(defaultAdmin, mintAmount, false, true);

      expect(await tokenMintingRateLimiter.mintedAmountInPeriod()).to.be.equal(0);
    });
  });

  describe("Role management", () => {
    it("can renounce DEFAULT_ADMIN_ROLE role and not grant roles", async () => {
      await tokenMintingRateLimiter.connect(minter).renounceRole(DEFAULT_ADMIN_ROLE, minterAddress);

      await expectRevertWithReason(
        tokenMintingRateLimiter.connect(minter).grantRole(DEFAULT_ADMIN_ROLE, minterAddress),
        buildAccessErrorMessage(minter, DEFAULT_ADMIN_ROLE),
      );
    });

    it("can renounce MINTER_ROLE role and not mint single address tokens", async () => {
      await tokenMintingRateLimiter.connect(minter).renounceRole(MINTER_ROLE, minterAddress);

      await expectRevertWithReason(
        tokenMintingRateLimiter.connect(minter).mint(deployerAddress, 1000n),
        buildAccessErrorMessage(minter, MINTER_ROLE),
      );
    });

    it("can renounce MINTER_ROLE role and not mint multiple address tokens", async () => {
      await tokenMintingRateLimiter.connect(minter).renounceRole(MINTER_ROLE, minterAddress);

      await expectRevertWithReason(
        tokenMintingRateLimiter.connect(minter).batchMint([deployerAddress], 1000n),
        buildAccessErrorMessage(minter, MINTER_ROLE),
      );
    });

    it("can renounce MINTER_ROLE role and not mint multiple address tokens with different amounts", async () => {
      await tokenMintingRateLimiter.connect(minter).renounceRole(MINTER_ROLE, minterAddress);

      await expect(
        tokenMintingRateLimiter.connect(minter).batchMintMultiple([deployerAddress], [1000n]),
      ).to.be.revertedWith("AccessControl: account " + minterAddress.toLowerCase() + " is missing role " + MINTER_ROLE);

      await expectRevertWithReason(
        tokenMintingRateLimiter.connect(minter).batchMintMultiple([deployerAddress], [1000n]),
        buildAccessErrorMessage(minter, MINTER_ROLE),
      );
    });
  });
});
