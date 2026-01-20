// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { YieldProviderVendor } from "./interfaces/YieldTypes.sol";

/**
 * @title Shared storage layout for the YieldManager and YieldProvider contracts.
 * @author Consensys Software Inc.
 * @dev Exposes the ERC-7201 storage schema consumed by `YieldManager` and the YieldProviders that it
        delegatecalls into.
 * @dev It is expected that the YieldManager performs the lion's share of state mutations, and YieldProviders
 *      step in only for a small subset of state which is intricately specific to a vendor.
 * @custom:security-contact security-report@linea.build
 */
abstract contract YieldManagerStorageLayout {
  /// @notice The Linea L1MessageService.
  address public immutable L1_MESSAGE_SERVICE;

  /**
   * @notice ERC-7201 namespaced storage layout for a YieldProvider adaptor contract
   * @custom:storage-location erc7201:linea.storage.YieldManagerStorage
   * @param minimumWithdrawalReservePercentageBps Minimum withdrawal reserve expressed as basis points of total user funds.
   * @param targetWithdrawalReservePercentageBps Target withdrawal reserve expressed as basis points of total user funds.
   * @param minimumWithdrawalReserveAmount Minimum withdrawal reserve expressed as an absolute number.
   * @param targetWithdrawalReserveAmount Target withdrawal reserve expressed as an absolute number.
   * @param userFundsInYieldProvidersTotal Funds owed to L1MessageService users which have been deposited across all YieldProviders.
   *                                        - Must increment and decrement 1:1 with YieldProviderStorage._userFunds.
   * @param pendingPermissionlessUnstake Pending ETH expected from permissionless beacon-chain withdrawal requests.
   *                                      - Greedily decremented on i.) Donations ii.) YieldProvider withdrawals.
   * @param yieldProviders Array of registered YieldProvider adaptor contracts.
   * @param isL2YieldRecipientKnown Mapping of added L2YieldRecipient addresses.
   * @param yieldProviderStorage YieldProvider-scoped storage scoped by the YieldProvider adaptor contract address.
   * @param lastProvenSlot Beacon chain validator index -> last proven slot.
   */
  struct YieldManagerStorage {
    uint16 minimumWithdrawalReservePercentageBps;
    uint16 targetWithdrawalReservePercentageBps;
    uint256 minimumWithdrawalReserveAmount;
    uint256 targetWithdrawalReserveAmount;
    uint256 userFundsInYieldProvidersTotal;
    uint256 pendingPermissionlessUnstake;
    address[] yieldProviders;
    mapping(address l2YieldRecipient => bool) isL2YieldRecipientKnown;
    mapping(address yieldProvider => YieldProviderStorage) yieldProviderStorage;
    mapping(uint64 validatorIndex => uint64 lastProvenSlot) lastProvenSlot;
  }

  /**
   * @notice ERC-7201 namespaced storage layout for a YieldProvider adaptor contract
   * @param yieldProviderVendor Specific type of YieldProvider adaptor.
   * @param isStakingPaused True if beacon chain deposits are paused
   * @param isOssificationInitiated True if ossification has been initiated. Remains true when ossification has finalized.
   * @param isOssified True if ossification has been finalized.
   * @param primaryEntrypoint Contract used for operations when not-ossified.
   * @param ossifiedEntrypoint Contract used for operations once ossification is finalized.
   * @param yieldProviderIndex Index for the YieldProvider.
   * @param userFunds Funds owed to L1MessageService users which have been deposited in the YieldProvider.
   *                  - Must increment and decrement 1:1 with _userFundsInYieldProvidersTotal.
   * @param yieldReportedCumulative Cumulative positive yield (denominated in ETH) reported back to the YieldManager.
   *                                - Increases 1:1 with userFunds, as reported yield is distributed to users.
   * @param lstLiabilityPrincipal LST Liability Principal (denominated in ETH) as of the last yield report
   * @param lastReportedNegativeYield Negative yield as of the last yield report.
   */
  struct YieldProviderStorage {
    // Slot 0: Packed fields (yieldProviderVendor, bools, primaryEntrypoint)
    YieldProviderVendor yieldProviderVendor;
    bool isStakingPaused;
    bool isOssificationInitiated;
    bool isOssified;
    address primaryEntrypoint;
    // Slot 1: Packed fields (ossifiedEntrypoint, yieldProviderIndex)
    address ossifiedEntrypoint;
    uint96 yieldProviderIndex;
    // Slots 2-5: Each uint256 occupies its own slot
    uint256 userFunds; // Slot 2
    uint256 yieldReportedCumulative; // Slot 3
    uint256 lstLiabilityPrincipal; // Slot 4
    uint256 lastReportedNegativeYield; // Slot 5
  }

  /// @dev keccak256(abi.encode(uint256(keccak256("linea.storage.YieldManagerStorage")) - 1)) & ~bytes32(uint256(0xff))
  bytes32 private constant YieldManagerStorageLocation =
    0xdc1272075efdca0b85fb2d76cbb5f26d954dc18e040d6d0b67071bd5cbd04300;

  /**
   * @notice Returns the ERC-7201 namespaced storage slot for the YieldManager global state.
   * @return $ Storage pointer granting read/write access to YieldManager global state.
   */
  function _getYieldManagerStorage() internal pure returns (YieldManagerStorage storage $) {
    assembly {
      $.slot := YieldManagerStorageLocation
    }
  }

  /**
   * @notice Returns the ERC-7201 namespaced storage slot for the YieldProvider-scoped state.
   * @param _yieldProvider YieldProvider adaptor address.
   * @return $$ Storage pointer granting read/write access to the YieldProvider-scoped state.
   */
  function _getYieldProviderStorage(address _yieldProvider) internal view returns (YieldProviderStorage storage $$) {
    $$ = _getYieldManagerStorage().yieldProviderStorage[_yieldProvider];
  }
}
