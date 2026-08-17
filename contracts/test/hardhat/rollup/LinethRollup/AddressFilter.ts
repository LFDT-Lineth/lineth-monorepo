import { SignerWithAddress } from "@nomicfoundation/hardhat-ethers/signers";
import { loadFixture } from "@nomicfoundation/hardhat-network-helpers";
import { expect } from "chai";
import { DEFAULT_ADMIN_ROLE } from "contracts/common/constants";
import { AddressFilter } from "contracts/typechain-types";

import { ADDRESS_ZERO } from "../../common/constants";
import {
  buildAccessErrorMessage,
  expectEvent,
  expectRevertWithCustomError,
  expectRevertWithReason,
} from "../../common/helpers";
import { getAccountsFixture, deployAddressFilterFixture, deployAddressFilter } from "../helpers";

describe("AddressFilter: Forced Transactions", () => {
  let addressFilter: AddressFilter;
  let securityCouncil: SignerWithAddress;
  let operator: SignerWithAddress;
  let nonAuthorizedAccount: SignerWithAddress;

  // let securityCouncil: SignerWithAddress;

  before(async () => {
    ({ nonAuthorizedAccount, securityCouncil, operator } = await loadFixture(getAccountsFixture));
  });

  beforeEach(async () => {
    ({ addressFilter } = await loadFixture(deployAddressFilterFixture));
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

      const hasRole = await addressFilter.hasRole(DEFAULT_ADMIN_ROLE, securityCouncil.address);
      expect(hasRole).to.be.true;
    });

    it("Should set the default address filter addresses", async () => {
      ({ addressFilter } = await deployAddressFilter(securityCouncil.address, [
        nonAuthorizedAccount.address,
        operator.address,
      ]));

      let isFiltered = await addressFilter.addressIsFiltered(nonAuthorizedAccount.address);
      expect(isFiltered).to.be.true;

      isFiltered = await addressFilter.addressIsFiltered(operator.address);
      expect(isFiltered).to.be.true;

      // double check it isn't just returning true
      isFiltered = await addressFilter.addressIsFiltered(securityCouncil.address);
      expect(isFiltered).to.be.false;
    });
  });

  describe("Setting address filter addresses statuses", () => {
    it("Should fail if the address list is empty", async () => {
      const asyncCall = addressFilter.connect(securityCouncil).setFilteredStatus([], true);

      await expectRevertWithCustomError(addressFilter, asyncCall, "FilteredAddressesEmpty");
    });

    it("Should set the address filter addresses to true", async () => {
      let isFiltered = await addressFilter.addressIsFiltered(operator.address);
      expect(isFiltered).to.be.false;

      await addressFilter.connect(securityCouncil).setFilteredStatus([operator.address], true);

      // double check it isn't just returning true
      isFiltered = await addressFilter.addressIsFiltered(operator.address);
      expect(isFiltered).to.be.true;
    });

    it("Should set the address filter addresses to false", async () => {
      let isFiltered = await addressFilter.addressIsFiltered(nonAuthorizedAccount.address);
      expect(isFiltered).to.be.true;

      await addressFilter.connect(securityCouncil).setFilteredStatus([nonAuthorizedAccount.address], false);

      // double check it isn't just returning false
      isFiltered = await addressFilter.addressIsFiltered(nonAuthorizedAccount.address);
      expect(isFiltered).to.be.false;
    });

    it("Should emit FilteredStatusesSet when setting status to true", async () => {
      await expectEvent(
        addressFilter,
        addressFilter.connect(securityCouncil).setFilteredStatus([operator.address], true),
        "FilteredStatusesSet",
        [[operator.address], true],
      );
    });

    it("Should emit FilteredStatusesSet when setting status to false", async () => {
      await expectEvent(
        addressFilter,
        addressFilter.connect(securityCouncil).setFilteredStatus([nonAuthorizedAccount.address], false),
        "FilteredStatusesSet",
        [[nonAuthorizedAccount.address], false],
      );
    });
  });

  describe("Access Control", () => {
    it("Should fail to set statuses if not authorized", async () => {
      await expectRevertWithReason(
        addressFilter.connect(nonAuthorizedAccount).setFilteredStatus([operator.address], true),
        buildAccessErrorMessage(nonAuthorizedAccount, DEFAULT_ADMIN_ROLE),
      );
    });
  });
});
