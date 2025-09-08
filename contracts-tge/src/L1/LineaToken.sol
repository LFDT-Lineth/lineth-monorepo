// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.30;

import {AccessControlUpgradeable} from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import {ERC20Upgradeable} from "@openzeppelin/contracts-upgradeable/token/ERC20/ERC20Upgradeable.sol";
import {ERC20BurnableUpgradeable} from "@openzeppelin/contracts-upgradeable/token/ERC20/extensions/ERC20BurnableUpgradeable.sol";
import {ERC20PermitUpgradeable} from "@openzeppelin/contracts-upgradeable/token/ERC20/extensions/ERC20PermitUpgradeable.sol";
import {IMessageService} from "../interfaces/IMessageService.sol";
import {ILineaToken} from "./interfaces/ILineaToken.sol";
import {IL2LineaToken} from "../L2/interfaces/IL2LineaToken.sol";

/**
 * @title Contract to manage the Linea Token.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract LineaToken is ERC20Upgradeable, ERC20BurnableUpgradeable, AccessControlUpgradeable, ERC20PermitUpgradeable, ILineaToken {
  /// @notice The role required to mint tokens.
  bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");

  /// @notice L1 message service contract address.
  address public l1MessageService;
  /// @notice L2 Linea token contract address.
  address public l2TokenAddress;

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes the contract.
   * @dev The default decimals of 18 applies to this token.
   * @param _defaultAdmin The default admin of the contract.
   * @param _minter A default token minter.
   * @param _l1MessageService The address of the L1 message service.
   * @param _l2TokenAddress The address of the L2 Linea token.
   * @param _tokenName The name of the token.
   * @param _tokenSymbol The symbol of the token.
   */
  function initialize(
    address _defaultAdmin,
    address _minter,
    address _l1MessageService,
    address _l2TokenAddress,
    string calldata _tokenName,
    string calldata _tokenSymbol
  ) external initializer {
    require(_defaultAdmin != address(0), ZeroAddressNotAllowed());
    require(_minter != address(0), ZeroAddressNotAllowed());
    require(_l1MessageService != address(0), ZeroAddressNotAllowed());
    require(_l2TokenAddress != address(0), ZeroAddressNotAllowed());
    require(bytes(_tokenName).length > 0, EmptyStringNotAllowed());
    require(bytes(_tokenSymbol).length > 0, EmptyStringNotAllowed());

    __ERC20_init(_tokenName, _tokenSymbol);
    __ERC20Burnable_init();
    __AccessControl_init();
    __ERC20Permit_init(_tokenName);

    _grantRole(DEFAULT_ADMIN_ROLE, _defaultAdmin);
    _grantRole(MINTER_ROLE, _minter);

    l1MessageService = _l1MessageService;
    l2TokenAddress = _l2TokenAddress;

    emit TokenMetadataSet(_tokenName, _tokenSymbol);
    emit L1MessageServiceSet(_l1MessageService);
    emit L2TokenAddressSet(_l2TokenAddress);
  }

  /**
   * @notice Mints the Linea token.
   * @dev NB: Only those with MINTER_ROLE can call this function.
   * @param _account Account being minted for.
   * @param _amount The amount being minted for the account.
   */
  function mint(address _account, uint256 _amount) external onlyRole(MINTER_ROLE) {
    _mint(_account, _amount);
  }

  /**
   * @notice Synchronizes the total supply of the L1 token to the L2 token.
   * @dev This function sends a message to the L2 token contract to sync the total supply.
   * @dev NB: This function is permissionless on purpose, allowing anyone to trigger the sync.
   * @dev This function can only be called after the L2 token address has been set.
   */
  function syncTotalSupplyToL2() external {
    uint256 totalSupply = totalSupply();

    /// @dev Fee is set to 0 and should be automatically claimed on Linea.
    IMessageService(l1MessageService).sendMessage(l2TokenAddress, 0, abi.encodeCall(IL2LineaToken.syncTotalSupplyFromL1, (block.timestamp, totalSupply)));

    emit L1TotalSupplySyncStarted(totalSupply);
  }
}
