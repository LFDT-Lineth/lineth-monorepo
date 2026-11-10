import { expect } from "chai";
import { network } from "hardhat";
import { describe, it, before, beforeEach } from "node:test";

import { DEFAULT_ADMIN_ROLE } from "../../../../common/constants/index.js";
import { ADDRESS_ZERO } from "../../common/constants/index.js";
import {
  type TestAccount,
  buildAccessErrorMessage,
  expectRevertWithCustomError,
  expectEvent,
  expectRevertWithReason,
} from "../../common/helpers/index.js";
import { getAccountsFixture, deployAddressFilterFixture, deployAddressFilter } from "../helpers/index.js";

import type { ContractReturnType } from "@nomicfoundation/hardhat-viem/types";

describe("AddressFilter: Forced Transactions", () => {
  let addressFilter: ContractReturnType<"AddressFilter">;
  let securityCouncil: TestAccount;
  let operator: TestAccount;
  let nonAuthorizedAccount: TestAccount;

  // let securityCouncil: TestAccount;

  before(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ nonAuthorizedAccount, securityCouncil, operator } = await networkHelpers.loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    const { networkHelpers } = await network.getOrCreate();
    ({ addressFilter } = await networkHelpers.loadFixture(deployAddressFilterFixture));
  });

  describe("Contract Construction", () => {
    it("Should fail if the _defaultAdmin is set as address(0)", async () => {
      await expectRevertWithCustomError(
        addressFilter,
        deployAddressFilter(ADDRESS_ZERO, [nonAuthorizedAccount.address]),
        "ZeroAddressNotAllowed",
      );
    });

    it("Should set the _defaultAdmin role", async () => {
      await deployAddressFilter(securityCouncil.address, [nonAuthorizedAccount.address]);

      const hasRole = await addressFilter.read.hasRole([DEFAULT_ADMIN_ROLE, securityCouncil.address]);
      expect(hasRole).to.be.true;
    });

    it("Should set the default address filter addresses", async () => {
      ({ addressFilter } = await deployAddressFilter(securityCouncil.address, [
        nonAuthorizedAccount.address,
        operator.address,
      ]));

      let isFiltered = await addressFilter.read.addressIsFiltered([nonAuthorizedAccount.address]);
      expect(isFiltered).to.be.true;

      isFiltered = await addressFilter.read.addressIsFiltered([operator.address]);
      expect(isFiltered).to.be.true;

      // double check it isn't just returning true
      isFiltered = await addressFilter.read.addressIsFiltered([securityCouncil.address]);
      expect(isFiltered).to.be.false;
    });
  });

  describe("Setting address filter addresses statuses", () => {
    it("Should fail if the address list is empty", async () => {
      const asyncCall = addressFilter.write.setFilteredStatus([[], true], { account: securityCouncil.account });

      await expectRevertWithCustomError(addressFilter, asyncCall, "FilteredAddressesEmpty");
    });

    it("Should set the address filter addresses to true", async () => {
      let isFiltered = await addressFilter.read.addressIsFiltered([operator.address]);
      expect(isFiltered).to.be.false;

      await addressFilter.write.setFilteredStatus([[operator.address], true], { account: securityCouncil.account });

      // double check it isn't just returning true
      isFiltered = await addressFilter.read.addressIsFiltered([operator.address]);
      expect(isFiltered).to.be.true;
    });

    it("Should set the address filter addresses to false", async () => {
      let isFiltered = await addressFilter.read.addressIsFiltered([nonAuthorizedAccount.address]);
      expect(isFiltered).to.be.true;

      await addressFilter.write.setFilteredStatus([[nonAuthorizedAccount.address], false], {
        account: securityCouncil.account,
      });

      // double check it isn't just returning false
      isFiltered = await addressFilter.read.addressIsFiltered([nonAuthorizedAccount.address]);
      expect(isFiltered).to.be.false;
    });

    it("Should emit FilteredStatusesSet when setting status to true", async () => {
      await expectEvent(
        addressFilter,
        addressFilter.write.setFilteredStatus([[operator.address], true], { account: securityCouncil.account }),
        "FilteredStatusesSet",
        [[operator.address], true],
      );
    });

    it("Should emit FilteredStatusesSet when setting status to false", async () => {
      await expectEvent(
        addressFilter,
        addressFilter.write.setFilteredStatus([[nonAuthorizedAccount.address], false], {
          account: securityCouncil.account,
        }),
        "FilteredStatusesSet",
        [[nonAuthorizedAccount.address], false],
      );
    });
  });

  describe("Access Control", () => {
    it("Should fail to set statuses if not authorized", async () => {
      await expectRevertWithReason(
        addressFilter.write.setFilteredStatus([[operator.address], true], { account: nonAuthorizedAccount.account }),
        buildAccessErrorMessage(nonAuthorizedAccount, DEFAULT_ADMIN_ROLE),
      );
    });
  });
});
