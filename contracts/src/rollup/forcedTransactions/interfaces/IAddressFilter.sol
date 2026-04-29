// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @title Interface to manage filtered addresses.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface IAddressFilter {
  /**
   * @notice Emitted when the filter state changes for one or more addresses.
   * @param filteredAddresses The addresses having their filter status set.
   * @param isFiltered The filtered status.
   */
  event FilteredStatusesSet(address[] filteredAddresses, bool isFiltered);

  /**
   * @dev Thrown when the address is already filtered.
   */
  error AddressAlreadyFiltered(address filteredAddress);

  /**
   * @dev Thrown when the address list to change filter status for is empty.
   */
  error FilteredAddressesEmpty();

  /**
   * @notice Function returning filtered status for an address.
   * @param _addressToCheck The address to check.
   * @return isFiltered The bool indicating if the address is on the filtered list.
   */
  function addressIsFiltered(address _addressToCheck) external view returns (bool isFiltered);

  /**
   * @notice Function to set one or more addresses to be filtered.
   * @dev Requires DEFAULT_ADMIN_ROLE.
   * @param _initialfilteredAddresses The address to set the status for.
   * @param _isFiltered The bool indicating if the address is on the filtered list.
   */
  function setFilteredStatus(address[] memory _initialfilteredAddresses, bool _isFiltered) external;
}
