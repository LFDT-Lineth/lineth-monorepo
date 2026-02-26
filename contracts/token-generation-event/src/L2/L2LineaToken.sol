// SPDX-License-Identifier: Apache-2.0 OR MIT

pragma solidity 0.8.30;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { ERC20Upgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/ERC20Upgradeable.sol";
import { ERC20PermitUpgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/extensions/ERC20PermitUpgradeable.sol";
import { ERC20VotesUpgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/extensions/ERC20VotesUpgradeable.sol";
import { NoncesUpgradeable } from "@openzeppelin/contracts-upgradeable/utils/NoncesUpgradeable.sol";

import { IL2LineaToken } from "../L2/interfaces/IL2LineaToken.sol";
import { MessageServiceBase } from "../MessageServiceBase.sol";

/**
 * @title Contract to manage the L2 Linea Token.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract L2LineaToken is
  ERC20Upgradeable,
  AccessControlUpgradeable,
  ERC20PermitUpgradeable,
  ERC20VotesUpgradeable,
  MessageServiceBase,
  IL2LineaToken
{
  /// @notice Linea Canonical Token Bridge contract address.
  address public lineaCanonicalTokenBridge;

  /// @notice L1 Linea Token total supply.
  uint256 public l1LineaTokenSupply;

  /// @notice The timestamp of the L1 block from which the total supply was synchronized.
  uint256 public l1LineaTokenTotalSupplySyncTime;

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes the contract.
   * @dev The default decimals of 18 applies to this token.
   * @param _defaultAdmin The default admin address of the contract.
   * @param _lineaCanonicalTokenBridge The address of the Linea Canonical Token Bridge.
   * @param _lineaMessageService The address of the Linea Message Service.
   * @param _l1Token The address of the Linea token on L1 Ethereum.
   * @param _tokenName The name of the token.
   * @param _tokenSymbol The symbol of the token.
   */
  function initialize(
    address _defaultAdmin,
    address _lineaCanonicalTokenBridge,
    address _lineaMessageService,
    address _l1Token,
    string calldata _tokenName,
    string calldata _tokenSymbol
  ) external initializer {
    require(_defaultAdmin != address(0), ZeroAddressNotAllowed());
    require(_lineaCanonicalTokenBridge != address(0), ZeroAddressNotAllowed());
    require(bytes(_tokenName).length > 0, EmptyStringNotAllowed());
    require(bytes(_tokenSymbol).length > 0, EmptyStringNotAllowed());

    __MessageServiceBase_init(_lineaMessageService);
    _setRemoteSender(_l1Token);

    __ERC20_init(_tokenName, _tokenSymbol);
    __AccessControl_init();
    __ERC20Permit_init(_tokenName);
    __ERC20Votes_init();

    _grantRole(DEFAULT_ADMIN_ROLE, _defaultAdmin);

    lineaCanonicalTokenBridge = _lineaCanonicalTokenBridge;

    emit L2MessageServiceSet(_lineaMessageService);
    emit L1TokenAddressSet(_l1Token);
    emit TokenMetadataSet(_tokenName, _tokenSymbol);
    emit LineaCanonicalTokenBridgeSet(_lineaCanonicalTokenBridge);
  }

  /**
   * @notice Mints the Linea token.
   * @dev NB: Only the L2 token bridge can call this function.
   * @param _account Account being minted for.
   * @param _amount The amount being minted for the account.
   */
  function mint(address _account, uint256 _amount) external {
    require(msg.sender == lineaCanonicalTokenBridge, CallerIsNotTokenBridge());

    _mint(_account, _amount);
  }

  /**
   * @notice Burns the Linea token.
   * @dev NB: Only the L2 token bridge can call this function.
   * @dev Approval for the burn amount must be provided before this is invoked.
   * @param _account Account being burned for.
   * @param _value The amount being burned for the account.
   */
  function burn(address _account, uint256 _value) external {
    require(msg.sender == lineaCanonicalTokenBridge, CallerIsNotTokenBridge());

    _spendAllowance(_account, msg.sender, _value);
    _burn(_account, _value);
  }

  /**
   * @notice Synchronizes the total supply of the L1 Linea token from L1 Ethereum.
   * @dev NB: This function can only be called by the Linea Message Service.
   * @dev NB: This function must have originated from the Linea token on L1 Ethereum.
   * @param _l1LineaTokenTotalSupplySyncTime The L1 block.timestamp when the Linea token on L1 total supply was
   * computed.
   * @param _l1LineaTokenSupply The total supply of the L1 Linea token.
   */
  function syncTotalSupplyFromL1(
    uint256 _l1LineaTokenTotalSupplySyncTime,
    uint256 _l1LineaTokenSupply
  ) external onlyMessagingService onlyAuthorizedRemoteSender {
    require(l1LineaTokenTotalSupplySyncTime < _l1LineaTokenTotalSupplySyncTime, LastSyncMoreRecent());

    l1LineaTokenSupply = _l1LineaTokenSupply;
    l1LineaTokenTotalSupplySyncTime = _l1LineaTokenTotalSupplySyncTime;

    emit L1LineaTokenTotalSupplySynced(_l1LineaTokenTotalSupplySyncTime, _l1LineaTokenSupply);
  }

  /**
   * @dev Returns the current nonce for `_owner`. This value must be
   * included whenever a signature is generated for {permit}.
   *
   * Every successful call to {permit} increases ``_owner``'s nonce by one. This
   * prevents a signature from being used multiple times.
   */
  function nonces(
    address _owner
  ) public view virtual override(ERC20PermitUpgradeable, NoncesUpgradeable) returns (uint256) {
    return super.nonces(_owner);
  }

  /**
   * @dev Transfers a `_value` amount of tokens from `_from` to `_to`, or alternatively mints (or burns) if `_from`
   * (or `_to`) is the zero address. All customizations to transfers, mints, and burns should be done by overriding
   * this function.
   * @dev Move voting power when tokens are transferred.
   *
   * Emits a {Transfer} event.
   * Emits a {IVotes-DelegateVotesChanged} event.
   */
  function _update(
    address _from,
    address _to,
    uint256 _value
  ) internal virtual override(ERC20Upgradeable, ERC20VotesUpgradeable) {
    super._update(_from, _to, _value);
  }
}
