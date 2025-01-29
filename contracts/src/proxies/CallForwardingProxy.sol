// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.26;

/**
 * @title Contract to forward calls to an underlying contract.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract CallForwardingProxy {
  /// @notice The underlying target address that is called.
  address public immutable TARGET;

  constructor(address _target) {
    TARGET = _target;
  }

  /**
   * @notice Defaults to, and forwards all calls to the target address.
   */
  fallback() external payable {
    (bool success, bytes memory data) = TARGET.call{ value: msg.value }(msg.data);
    require(success, "Call failed");

    assembly {
      return(add(data, 0x20), mload(data))
    }
  }

  receive() external payable {
    revert("ETH not accepted");
  }
}
