import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { ethers } from "hardhat";
import { LineaSurgeXP, TestLineaSurgeXP } from "../../../typechain-types";
import { ADDRESS_ZERO, DEFAULT_ADMIN_ROLE, MINTER_ROLE, TRANSFER_ROLE } from "../common/constants";
import { deployFromFactory } from "../common/deployment";

describe("Linea Surge XP Token Tests", () => {
  let contract: LineaSurgeXP;
  let transferContract: TestLineaSurgeXP;
  let admin: SignerWithAddress;
  let minter: SignerWithAddress;
  let deployer: SignerWithAddress;
  let nonTransferer: SignerWithAddress;

  async function deployLineaLXPLFixture() {
    return deployFromFactory("LineaSurgeXP", admin.address, minter.address, [
      minter.address,
      deployer.address,
    ]) as Promise<LineaSurgeXP>;
  }

  async function deployTestLineaSurgeXPFixture() {
    return deployFromFactory("TestLineaSurgeXP") as Promise<TestLineaSurgeXP>;
  }

  before(async () => {
    [deployer, admin, minter, nonTransferer] = await ethers.getSigners();
    transferContract = await loadFixture(deployTestLineaSurgeXPFixture);
  });

  beforeEach(async () => {
    contract = await loadFixture(deployLineaLXPLFixture);
  });

  describe("Initialization and roles", () => {
    it("fails to initialize when admin address is zero address", async () => {
      await expect(
        deployFromFactory("LineaSurgeXP", ADDRESS_ZERO, minter.address, [minter.address, deployer.address]),
      ).to.be.revertedWithCustomError(contract, "ZeroAddressNotAllowed");
    });

    it("fails to initialize when minter address is zero address", async () => {
      await expect(
        deployFromFactory("LineaSurgeXP", admin.address, ADDRESS_ZERO, [minter.address, deployer.address]),
      ).to.be.revertedWithCustomError(contract, "ZeroAddressNotAllowed");
    });

    it("fails to initialize when a transferer address is zero address", async () => {
      await expect(
        deployFromFactory("LineaSurgeXP", admin.address, minter.address, [minter.address, ADDRESS_ZERO]),
      ).to.be.revertedWithCustomError(contract, "ZeroAddressNotAllowed");
    });

    it("admin should have the 'DEFAULT_ADMIN_ROLE' role", async () => {
      expect(await contract.hasRole(DEFAULT_ADMIN_ROLE, admin.address)).to.be.true;
    });

    it("minter should have the 'MINTER_ROLE' role", async () => {
      expect(await contract.hasRole(MINTER_ROLE, minter.address)).to.be.true;
    });

    it("minter should have the 'TRANSFER_ROLE' role", async () => {
      expect(await contract.hasRole(TRANSFER_ROLE, minter.address)).to.be.true;
    });

    it("deployer should have the 'TRANSFER_ROLE' role", async () => {
      expect(await contract.hasRole(TRANSFER_ROLE, deployer.address)).to.be.true;
    });

    it("deployer should NOT have the 'MINTER_ROLE' role", async () => {
      expect(await contract.hasRole(MINTER_ROLE, deployer.address)).to.be.false;
    });

    it("deployer should NOT have the 'DEFAULT_ADMIN_ROLE' role", async () => {
      expect(await contract.hasRole(DEFAULT_ADMIN_ROLE, deployer.address)).to.be.false;
    });
  });

  describe("Single minting", () => {
    it("non-minter cannot mint tokens", async () => {
      await expect(contract.connect(deployer).mint(deployer.address, 1000n)).to.be.revertedWith(
        "AccessControl: account " + deployer.address.toLowerCase() + " is missing role " + MINTER_ROLE,
      );
    });

    it("minter can mint tokens", async () => {
      await contract.connect(minter).mint(deployer.address, 1000n);

      expect(await contract.balanceOf(deployer.address)).to.be.equal(1000n);
    });
  });

  describe("Batch minting with one amount", () => {
    it("non-minter cannot mint tokens", async () => {
      await expect(contract.batchMint([deployer.address], 1000n)).to.be.revertedWith(
        "AccessControl: account " + deployer.address.toLowerCase() + " is missing role " + MINTER_ROLE,
      );
    });

    it("minter can mint tokens for one address", async () => {
      await contract.connect(minter).batchMint([deployer.address], 1000n);

      expect(await contract.balanceOf(deployer.address)).to.be.equal(1000n);
    });

    it("minter can mint tokens for multiple address", async () => {
      await contract.connect(minter).batchMint([minter.address, deployer.address], 1000n);

      expect(await contract.balanceOf(deployer.address)).to.be.equal(1000n);
      expect(await contract.balanceOf(minter.address)).to.be.equal(1000n);
    });
  });

  describe("Batch minting with varying amounts", () => {
    it("non-minter cannot mint tokens", async () => {
      await expect(contract.batchMintMultiple([deployer.address], [1000n])).to.be.revertedWith(
        "AccessControl: account " + deployer.address.toLowerCase() + " is missing role " + MINTER_ROLE,
      );
    });

    it("minter can mint tokens for one address", async () => {
      await contract.connect(minter).batchMintMultiple([deployer.address], [1000n]);

      expect(await contract.balanceOf(deployer.address)).to.be.equal(1000n);
    });

    it("minter can mint tokens for multiple address with different amounts", async () => {
      await contract.connect(minter).batchMintMultiple([minter.address, deployer.address], [1000n, 2000n]);

      expect(await contract.balanceOf(deployer.address)).to.be.equal(2000n);
      expect(await contract.balanceOf(minter.address)).to.be.equal(1000n);
    });

    it("cannot mint when array lengths are different", async () => {
      await expect(
        contract.connect(minter).batchMintMultiple([minter.address, deployer.address], [1000n]),
      ).to.be.revertedWith("Array lengths do not match");
    });
  });

  describe("Tokens are SoulBound Functions", () => {
    it("cannot approve token amounts", async () => {
      await expect(contract.connect(deployer).approve(minter.address, 1000n)).to.be.revertedWithCustomError(
        contract,
        "TokenIsSoulBound",
      );
    });

    it("cannot transfer from allowance amounts", async () => {
      await expect(
        contract.connect(deployer).transferFrom(deployer.address, minter.address, 1000n),
      ).to.be.revertedWithCustomError(contract, "TokenIsSoulBound");
    });

    it("cannot increase token allowance amounts", async () => {
      await expect(contract.connect(deployer).increaseAllowance(minter.address, 1000n)).to.be.revertedWithCustomError(
        contract,
        "TokenIsSoulBound",
      );
    });

    it("cannot decrease token allowance amounts", async () => {
      await expect(contract.connect(deployer).decreaseAllowance(minter.address, 1000n)).to.be.revertedWithCustomError(
        contract,
        "TokenIsSoulBound",
      );
    });
  });

  describe("Role management", () => {
    it("can renounce DEFAULT_ADMIN_ROLE role and not grant roles", async () => {
      await contract.connect(minter).renounceRole(DEFAULT_ADMIN_ROLE, minter.address);

      await expect(contract.connect(minter).grantRole(DEFAULT_ADMIN_ROLE, minter.address)).to.be.revertedWith(
        "AccessControl: account " + minter.address.toLowerCase() + " is missing role " + DEFAULT_ADMIN_ROLE,
      );
    });

    it("can renounce MINTER_ROLE role and not mint single address tokens", async () => {
      await contract.connect(minter).renounceRole(MINTER_ROLE, minter.address);

      await expect(contract.connect(minter).mint(deployer.address, 1000n)).to.be.revertedWith(
        "AccessControl: account " + minter.address.toLowerCase() + " is missing role " + MINTER_ROLE,
      );
    });

    it("can renounce MINTER_ROLE role and not mint multiple address tokens", async () => {
      await contract.connect(minter).renounceRole(MINTER_ROLE, minter.address);

      await expect(contract.connect(minter).batchMint([deployer.address], 1000n)).to.be.revertedWith(
        "AccessControl: account " + minter.address.toLowerCase() + " is missing role " + MINTER_ROLE,
      );
    });

    it("can renounce MINTER_ROLE role and not mint multiple address tokens with different amounts", async () => {
      await contract.connect(minter).renounceRole(MINTER_ROLE, minter.address);

      await expect(contract.connect(minter).batchMintMultiple([deployer.address], [1000n])).to.be.revertedWith(
        "AccessControl: account " + minter.address.toLowerCase() + " is missing role " + MINTER_ROLE,
      );
    });

    it("cannot transfer token amounts if no role", async () => {
      await expect(contract.connect(nonTransferer).transfer(minter.address, 1000n)).to.be.revertedWith(
        "AccessControl: account " + nonTransferer.address.toLowerCase() + " is missing role " + TRANSFER_ROLE,
      );
    });

    it("can grant TRANSFER_ROLE roles", async () => {
      await contract.connect(admin).grantRole(TRANSFER_ROLE, deployer.address);

      expect(await contract.hasRole(TRANSFER_ROLE, deployer.address)).true;
    });
  });

  describe("Transferring", () => {
    it("Fails to transfer as account", async () => {
      await contract.connect(admin).grantRole(TRANSFER_ROLE, deployer.address);

      await expect(contract.connect(deployer).transfer(minter.address, 100n)).to.be.revertedWithCustomError(
        contract,
        "CallerIsNotContract",
      );
    });

    it("can transfer from allowed contract", async () => {
      const transferContractAddress = await transferContract.getAddress();
      const contractAddress = await contract.getAddress();

      await contract.connect(minter).mint(transferContractAddress, 1000n);
      await contract.connect(admin).grantRole(TRANSFER_ROLE, transferContractAddress);

      const minterBalanceBefore = await contract.balanceOf(minter.address);

      await transferContract.testTransfer(contractAddress, minter.address, 1000n);

      const expectedBalance = minterBalanceBefore + 1000n;

      expect(await contract.balanceOf(minter.address)).equal(expectedBalance);
    });
  });

  describe.skip("Gas limitations", () => {
    it("Can mint for 1000 addresses with the same amount", async () => {
      const randomWallets = [];
      for (let i = 0; i < 1000; i++) {
        randomWallets.push(ethers.Wallet.createRandom().address);
      }

      await contract.connect(minter).batchMint(randomWallets, 1000n, { gasLimit: 26000000 });
    }).timeout(200_000);

    it("Can mint for 1000 addresses with the different amounts amount", async () => {
      const randomWallets = [];
      const amounts = [];

      for (let i = 0; i < 1000; i++) {
        randomWallets.push(ethers.Wallet.createRandom().address);
        amounts.push(BigInt(i + 100));
      }

      await contract.connect(minter).batchMintMultiple(randomWallets, amounts, { gasLimit: 26000000 });
    }).timeout(200_000);
  });
});
