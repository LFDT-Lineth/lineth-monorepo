// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { IGenericErrors } from "../interfaces/IGenericErrors.sol";
import { IRecoverFunds } from "./interfaces/IRecoverFunds.sol";

/**
 * @title Contract to recover funds sent to the message service address on the alternate chain.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract RecoverFunds is AccessControlUpgradeable, IRecoverFunds {
  bytes32 public constant FUNCTION_EXECUTOR_ROLE = keccak256("FUNCTION_EXECUTOR_ROLE");

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initialises underlying dependencies and sets initial access control.
   * @param _securityCouncil The address owning the security council role.
   * @param _executorAddress The executeExternalCall executor address.
   */
  function initialize(address _securityCouncil, address _executorAddress) external initializer {
    if (_securityCouncil == address(0)) {
      revert IGenericErrors.ZeroAddressNotAllowed();
    }

    if (_executorAddress == address(0)) {
      revert IGenericErrors.ZeroAddressNotAllowed();
    }

    __AccessControl_init();

    _grantRole(DEFAULT_ADMIN_ROLE, _securityCouncil);
    _grantRole(FUNCTION_EXECUTOR_ROLE, _executorAddress);
  }

  /**
   * @notice Executes external calls.
   * @param _destination The address being called.
   * @param _callData The calldata being sent to the address.
   * @param _ethValue Any ETH value being sent.
   * @dev "0x" for calldata can be used for simple ETH transfers.
   */
  function executeExternalCall(
    address _destination,
    bytes memory _callData,
    uint256 _ethValue
  ) external payable onlyRole(FUNCTION_EXECUTOR_ROLE) {
    (bool callSuccess, bytes memory returnData) = _destination.call{ value: _ethValue }(_callData);
    if (!callSuccess) {
      if (returnData.length > 0) {
        assembly {
          let data_size := mload(returnData)
          revert(add(32, returnData), data_size)
        }
      } else {
        revert ExternalCallFailed(_destination);
      }
    }
  }
}
