// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;
import { IGenericErrors } from "../../interfaces/IGenericErrors.sol";
import { IAddressFilter } from "./interfaces/IAddressFilter.sol";
import { AccessControl } from "@openzeppelin/contracts/access/AccessControl.sol";

/**
 * @title Contract to manage filtered addresses on L1.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract AddressFilter is AccessControl, IAddressFilter {
  mapping(address ethereumAddress => bool isFiltered) internal filteredAddresses;

  constructor(address _defaultAdmin, address[] memory _initialfilteredAddresses) {
    require(_defaultAdmin != address(0), IGenericErrors.ZeroAddressNotAllowed());

    _grantRole(DEFAULT_ADMIN_ROLE, _defaultAdmin);
    _setFilteredStatus(_initialfilteredAddresses, true);
  }

  /**
   * @notice Function returning filtered status for an address.
   * @param _addressToCheck The address to check.
   * @return isFiltered The bool indicating if the address is on the filtered list.
   */
  function addressIsFiltered(address _addressToCheck) external view returns (bool isFiltered) {
    isFiltered = filteredAddresses[_addressToCheck];
  }

  /**
   * @notice Function to set one or more addresses to be filtered.
   * @dev Requires DEFAULT_ADMIN_ROLE.
   * @param _initialfilteredAddresses The address to set the status for.
   * @param _isFiltered The bool indicating if the address is on the filtered list.
   */
  function setFilteredStatus(
    address[] memory _initialfilteredAddresses,
    bool _isFiltered
  ) external onlyRole(DEFAULT_ADMIN_ROLE) {
    require(_initialfilteredAddresses.length > 0, FilteredAddressesEmpty());
    _setFilteredStatus(_initialfilteredAddresses, _isFiltered);
  }

  /**
   * @notice Internal function to set one or more addresses to be filtered.
   * @param _initialfilteredAddresses The address to set the status for.
   * @param _isFiltered The bool indicating if the address is on the filtered list.
   */
  function _setFilteredStatus(address[] memory _initialfilteredAddresses, bool _isFiltered) internal {
    uint256 cachedAddressesLength = _initialfilteredAddresses.length;
    for (uint256 i; i < cachedAddressesLength; i++) {
      filteredAddresses[_initialfilteredAddresses[i]] = _isFiltered;
    }

    emit FilteredStatusesSet(_initialfilteredAddresses, _isFiltered);
  }
}
