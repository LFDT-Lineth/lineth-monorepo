// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;
import { L2MessageServiceBase } from "./L2MessageServiceBase.sol";
/**
 * @title Contract to manage cross-chain messaging on L2.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract L2MessageService is L2MessageServiceBase {
  /// @dev Total contract storage is 50 slots with the gap below.
  /// @dev Keep 50 free storage slots for future implementation updates to avoid storage collision.
  uint256[50] private __gap_L2MessageService;

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes underlying message service dependencies.
   * @param _rateLimitPeriod The period to rate limit against.
   * @param _rateLimitAmount The limit allowed for withdrawing the period.
   * @param _defaultAdmin The account to be given DEFAULT_ADMIN_ROLE on initialization.
   * @param _roleAddresses The list of addresses to grant roles to.
   * @param _pauseTypeRoleAssignments The list of pause type roles.
   * @param _unpauseTypeRoleAssignments The list of unpause type roles.
   */
  function initialize(
    uint256 _rateLimitPeriod,
    uint256 _rateLimitAmount,
    address _defaultAdmin,
    RoleAddress[] calldata _roleAddresses,
    PauseTypeRole[] calldata _pauseTypeRoleAssignments,
    PauseTypeRole[] calldata _unpauseTypeRoleAssignments
  ) external virtual initializer {
    __L2MessageService_init(
      _rateLimitPeriod,
      _rateLimitAmount,
      _defaultAdmin,
      _roleAddresses,
      _pauseTypeRoleAssignments,
      _unpauseTypeRoleAssignments
    );
  }

  /**
   * @notice Reinitializes the L2MessageService and clears the old reentry slot value.
   */
  function reinitializeV3() external reinitializer(3) {
    address proxyAdmin;
    assembly {
      proxyAdmin := sload(PROXY_ADMIN_SLOT)
    }
    require(msg.sender == proxyAdmin, CallerNotProxyAdmin());

    assembly {
      sstore(177, 0)
    }
  }
}
