// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { L2MessageServiceV1 } from "./v1/L2MessageServiceV1.sol";
import { L2MessageManager } from "./L2MessageManager.sol";
import { PermissionsManager } from "../../security/access/PermissionsManager.sol";

/**
 * @title Contract to manage cross-chain messaging on L2.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract L2MessageService is AccessControlUpgradeable, L2MessageServiceV1, L2MessageManager, PermissionsManager {
  /// @dev This is the ABI version and not the reinitialize version.
  string public constant CONTRACT_VERSION = "1.0";

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
   * @param _pauseTypeRoles The list of pause type roles.
   * @param _unpauseTypeRoles The list of unpause type roles.
   */
  function initialize(
    uint256 _rateLimitPeriod,
    uint256 _rateLimitAmount,
    address _defaultAdmin,
    RoleAddress[] calldata _roleAddresses,
    PauseTypeRole[] calldata _pauseTypeRoles,
    PauseTypeRole[] calldata _unpauseTypeRoles
  ) external initializer {
    __ERC165_init();
    __Context_init();
    __AccessControl_init();
    __RateLimiter_init(_rateLimitPeriod, _rateLimitAmount);

    __ReentrancyGuard_init();
    __PauseManager_init(_pauseTypeRoles, _unpauseTypeRoles);

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
   * @notice Sets permissions for a list of addresses and their roles as well as initialises the PauseManager pauseType:role mappings.
   * @dev This function is a reinitializer and can only be called once per version. Should be called using an upgradeAndCall transaction to the ProxyAdmin.
   * @param _roleAddresses The list of addresses and roles to assign permissions to.
   * @param _pauseTypeRoles The list of pause types to associate with roles.
   * @param _unpauseTypeRoles The list of unpause types to associate with roles.
   */
  function reinitializePauseTypesAndPermissions(
    RoleAddress[] calldata _roleAddresses,
    PauseTypeRole[] calldata _pauseTypeRoles,
    PauseTypeRole[] calldata _unpauseTypeRoles
  ) external reinitializer(2) {
    __Permissions_init(_roleAddresses);
    __PauseManager_init(_pauseTypeRoles, _unpauseTypeRoles);
  }
}
