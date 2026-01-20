// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;

import { TokenBridgeBase } from "../../../bridging/token/TokenBridgeBase.sol";

contract InheritingL1TokenBridge is TokenBridgeBase {
  enum BridingStatus {
    REQUESTED,
    READY,
    COMPLETE
  }

  mapping(address tokenAddress => address escrowAddress) escrowAddresses;
  mapping(bytes32 bridgingHash => BridingStatus status) public bridgingStatuses;

  function initialize(
    InitializationData calldata _initializationData
  )
    external
    nonZeroAddress(_initializationData.messageService)
    nonZeroAddress(_initializationData.tokenBeacon)
    nonZeroChainId(_initializationData.sourceChainId)
    nonZeroChainId(_initializationData.targetChainId)
    initializer
  {
    // New custom initialization behavior here.
    __TokenBridge_init(_initializationData);
  }

  function _getEscrowAddress(address _token) internal view virtual override returns (address escrowAddress) {
    // Overriden to allow the movement of specific assets to custom escrow services.

    escrowAddress = escrowAddresses[_token];
    if (escrowAddress == address(0)) {
      escrowAddress = address(this);
    }
  }

  // Implementor to add security.
  function setEscrowAddress(address _token, address _escrowAddress) external {
    // Validate fields
    // TBC check not already set?
    escrowAddresses[_token] = _escrowAddress;

    // event emitted
  }

  function finalizeBridging(
    address _nativeToken,
    uint256 _amount,
    address _recipient,
    uint256 _chainId,
    bytes calldata _tokenMetadata
  ) external {
    // New custom function to finalize withdrawal if a multi-step approach is taken.
  }

  function _completeBridging(
    address _nativeToken,
    uint256 _amount,
    address _recipient,
    uint256 _chainId,
    bytes calldata _tokenMetadata
  ) internal virtual override {
    // Similar approach to the ETH bridge with hashing and withdrawal flow management.
  }
}
