// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

/**
 * @notice Enum defining the specific type of YieldProvider adaptor.
 */
enum YieldProviderVendor {
  UNUSED,
  LIDO_STVAULT
}

/// @notice Enum defining the outcome of progressPendingOssification
enum ProgressOssificationResult {
  REINITIATED,
  NOOP,
  COMPLETE
}

/**
 * @notice Struct representing expected information to add a YieldProvider adaptor instance.
 * @param yieldProviderVendor Specific type of YieldProvider adaptor.
 * @param primaryEntrypoint Contract used for operations when not-ossified.
 * @param ossifiedEntrypoint Contract used for operations once ossification is finalized.
 * @param usersFundsIncrement Initial amount of userFunds that should be accounted for when registering the yield provider.
 */
struct YieldProviderRegistration {
  YieldProviderVendor yieldProviderVendor;
  address primaryEntrypoint;
  address ossifiedEntrypoint;
  uint256 usersFundsIncrement;
}
