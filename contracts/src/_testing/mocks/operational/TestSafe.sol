// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

import { ISafe } from "../../../operational/interfaces/ISafe.sol";

/**
 * @title Test double for a multisig Safe.
 * @notice Implements the minimal {ISafe} surface and can forward an arbitrary call so tests can assert that
 * downstream conditions rely on `tx.origin` (the EOA) rather than `msg.sender` (this contract).
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract TestSafe is ISafe {
  mapping(address owner => bool isRegistered) private _owners;

  /**
   * @notice Registers the initial set of Safe owners.
   * @param _initialOwners The addresses to register as owners.
   */
  constructor(address[] memory _initialOwners) {
    uint256 length = _initialOwners.length;
    for (uint256 i; i < length; i++) {
      _owners[_initialOwners[i]] = true;
    }
  }

  /**
   * @notice Sets the ownership flag for an address.
   * @param _owner The address whose ownership flag is updated.
   * @param _isOwner The ownership flag to assign.
   */
  function setOwner(address _owner, bool _isOwner) external {
    _owners[_owner] = _isOwner;
  }

  /**
   * @notice Forwards a call to `_target` so callees observe this contract as `msg.sender`.
   * @param _target The contract to call.
   * @param _data The calldata to forward.
   */
  function execute(address _target, bytes calldata _data) external {
    (bool success, bytes memory returnData) = _target.call(_data);
    if (!success) {
      assembly {
        revert(add(returnData, 0x20), mload(returnData))
      }
    }
  }

  /**
   * @inheritdoc ISafe
   */
  function isOwner(address _owner) external view returns (bool isAnOwner) {
    return _owners[_owner];
  }
}
