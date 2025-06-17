// SPDX-License-Identifier: AGPL-3.0
pragma solidity ^0.8.30;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { L2MessageServiceV1 } from "./v1/L2MessageServiceV1.sol";
import { L2MessageManager } from "./L2MessageManager.sol";
import { PermissionsManager } from "../../security/access/PermissionsManager.sol";

/**
 * @title Contract to manage cross-chain messaging on L2.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
abstract contract L2MessageServiceBase is
  AccessControlUpgradeable,
  L2MessageServiceV1,
  L2MessageManager,
  PermissionsManager
{
  /// @dev This is the ABI version and not the reinitialize version.
  string private constant _CONTRACT_VERSION = "1.0";

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
  function __L2MessageService_init(
    uint256 _rateLimitPeriod,
    uint256 _rateLimitAmount,
    address _defaultAdmin,
    RoleAddress[] calldata _roleAddresses,
    PauseTypeRole[] calldata _pauseTypeRoleAssignments,
    PauseTypeRole[] calldata _unpauseTypeRoleAssignments
  ) internal virtual {
    __ERC165_init();
    __Context_init();
    __AccessControl_init();
    __RateLimiter_init(_rateLimitPeriod, _rateLimitAmount);

    __PauseManager_init(_pauseTypeRoleAssignments, _unpauseTypeRoleAssignments);
    __ReentrancyGuard_init();

    if (_defaultAdmin == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    /**
     * @dev DEFAULT_ADMIN_ROLE is set for the security council explicitly,
     * as the permissions init purposefully does not allow DEFAULT_ADMIN_ROLE to be set.
     */
    _grantRole(DEFAULT_ADMIN_ROLE, _defaultAdmin);

    __Permissions_init(_roleAddresses);

    nextMessageNumber = 1;

    _messageSender = DEFAULT_SENDER_ADDRESS;
    minimumFeeInWei = 0.0001 ether;
  }

  /**
   * @notice Returns the ABI version and not the reinitialize version.
   * @return contractVersion The contract ABI version.
   */
  function CONTRACT_VERSION() external view virtual returns (string memory contractVersion) {
    contractVersion = _CONTRACT_VERSION;
  }
}
