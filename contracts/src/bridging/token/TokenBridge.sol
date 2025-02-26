// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.19;

import { ITokenBridge } from "./interfaces/ITokenBridge.sol";
import { IMessageService } from "../../messaging/interfaces/IMessageService.sol";

import { IERC20PermitUpgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/extensions/IERC20PermitUpgradeable.sol";
import { IERC20MetadataUpgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/extensions/IERC20MetadataUpgradeable.sol";
import { IERC20Upgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/IERC20Upgradeable.sol";
import { SafeERC20Upgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/utils/SafeERC20Upgradeable.sol";
import { BeaconProxy } from "@openzeppelin/contracts/proxy/beacon/BeaconProxy.sol";
import { ReentrancyGuardUpgradeable } from "@openzeppelin/contracts-upgradeable/security/ReentrancyGuardUpgradeable.sol";

import { BridgedToken } from "./BridgedToken.sol";
import { MessageServiceBase } from "../../messaging/MessageServiceBase.sol";

import { TokenBridgePauseManager } from "../../security/pausing/TokenBridgePauseManager.sol";
import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { StorageFiller39 } from "./utils/StorageFiller39.sol";
import { PermissionsManager } from "../../security/access/PermissionsManager.sol";

import { EfficientLeftRightKeccak } from "../../libraries/EfficientLeftRightKeccak.sol";
/**
 * @title Linea Canonical Token Bridge
 * @notice Contract to manage cross-chain ERC-20 bridging.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract TokenBridge is
  ITokenBridge,
  ReentrancyGuardUpgradeable,
  AccessControlUpgradeable,
  MessageServiceBase,
  TokenBridgePauseManager,
  PermissionsManager,
  StorageFiller39
{
  using EfficientLeftRightKeccak for *;
  using SafeERC20Upgradeable for IERC20Upgradeable;

  /// @dev This is the ABI version and not the reinitialize version.
  string public constant CONTRACT_VERSION = "1.0";

  /// @notice Role used for setting the message service address.
  bytes32 public constant SET_MESSAGE_SERVICE_ROLE = keccak256("SET_MESSAGE_SERVICE_ROLE");

  /// @notice Role used for setting the remote token bridge address.
  bytes32 public constant SET_REMOTE_TOKENBRIDGE_ROLE = keccak256("SET_REMOTE_TOKENBRIDGE_ROLE");

  /// @notice Role used for setting a reserved token address.
  bytes32 public constant SET_RESERVED_TOKEN_ROLE = keccak256("SET_RESERVED_TOKEN_ROLE");

  /// @notice Role used for removing a reserved token address.
  bytes32 public constant REMOVE_RESERVED_TOKEN_ROLE = keccak256("REMOVE_RESERVED_TOKEN_ROLE");

  /// @notice Role used for setting a custom token contract address.
  bytes32 public constant SET_CUSTOM_CONTRACT_ROLE = keccak256("SET_CUSTOM_CONTRACT_ROLE");

  // Special addresses used in the mappings to mark specific states for tokens.
  /// @notice EMPTY means a token is not present in the mapping.
  address internal constant EMPTY = address(0x0);
  /// @notice RESERVED means a token is reserved and cannot be bridged.
  address internal constant RESERVED_STATUS = address(0x111);
  /// @notice NATIVE means a token is native to the current local chain.
  address internal constant NATIVE_STATUS = address(0x222);
  /// @notice DEPLOYED means the bridged token contract has been deployed on the remote chain.
  address internal constant DEPLOYED_STATUS = address(0x333);

  // solhint-disable-next-line var-name-mixedcase
  /// @dev The permit selector to be used when decoding the permit.
  bytes4 internal constant _PERMIT_SELECTOR = IERC20PermitUpgradeable.permit.selector;

  /// @notice These 3 variables are used for the token metadata.
  bytes private constant METADATA_NAME = abi.encodeCall(IERC20MetadataUpgradeable.name, ());
  bytes private constant METADATA_SYMBOL = abi.encodeCall(IERC20MetadataUpgradeable.symbol, ());
  bytes private constant METADATA_DECIMALS = abi.encodeCall(IERC20MetadataUpgradeable.decimals, ());

  /// @dev These 3 values are used when checking for token decimals and string values.
  uint256 private constant VALID_DECIMALS_ENCODING_LENGTH = 32;
  uint256 private constant SHORT_STRING_ENCODING_LENGTH = 32;
  uint256 private constant MINIMUM_STRING_ABI_DECODE_LENGTH = 64;

  /// @notice The token beacon for deployed tokens.
  address public tokenBeacon;

  /// @notice The chainId mapped to a native token address which is then mapped to the bridged token address.
  mapping(uint256 chainId => mapping(address native => address bridged)) public nativeToBridgedToken;

  /// @notice The bridged token address mapped to the native token address.
  mapping(address bridged => address native) public bridgedToNativeToken;

  /// @notice The current layer's chainId from where the bridging is triggered.
  uint256 public sourceChainId;

  /// @notice The targeted layer's chainId where the bridging is received.
  uint256 public targetChainId;

  /// @dev Keep free storage slots for future implementation updates to avoid storage collision.
  uint256[50] private __gap;

  /// @dev Ensures the token has not been bridged before.
  modifier isNewToken(address _token) {
    if (bridgedToNativeToken[_token] != EMPTY || nativeToBridgedToken[sourceChainId][_token] != EMPTY)
      revert AlreadyBridgedToken(_token);
    _;
  }

  /**
   * @dev Ensures the address is not address(0).
   * @param _addr Address to check.
   */
  modifier nonZeroAddress(address _addr) {
    if (_addr == EMPTY) revert ZeroAddressNotAllowed();
    _;
  }

  /**
   * @dev Ensures the amount is not 0.
   * @param _amount amount to check.
   */
  modifier nonZeroAmount(uint256 _amount) {
    if (_amount == 0) revert ZeroAmountNotAllowed(_amount);
    _;
  }

  /**
   * @dev Ensures the chainId is not 0.
   * @param _chainId chainId to check.
   */
  modifier nonZeroChainId(uint256 _chainId) {
    if (_chainId == 0) revert ZeroChainIdNotAllowed();
    _;
  }

  /// @dev Disable constructor for safety
  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes TokenBridge and underlying service dependencies - used for new networks only.
   * @dev Contract will be used as proxy implementation.
   * @param _initializationData The initial data used for initializing the TokenBridge contract.
   */
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
    __PauseManager_init(_initializationData.pauseTypeRoles, _initializationData.unpauseTypeRoles);
    __MessageServiceBase_init(_initializationData.messageService);
    __ReentrancyGuard_init();

    if (_initializationData.defaultAdmin == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    /**
     * @dev DEFAULT_ADMIN_ROLE is set for the security council explicitly,
     * as the permissions init purposefully does not allow DEFAULT_ADMIN_ROLE to be set.
     */
    _grantRole(DEFAULT_ADMIN_ROLE, _initializationData.defaultAdmin);

    __Permissions_init(_initializationData.roleAddresses);

    tokenBeacon = _initializationData.tokenBeacon;
    if (_initializationData.sourceChainId == _initializationData.targetChainId) revert SourceChainSameAsTargetChain();
    sourceChainId = _initializationData.sourceChainId;
    targetChainId = _initializationData.targetChainId;

    unchecked {
      for (uint256 i; i < _initializationData.reservedTokens.length; ) {
        if (_initializationData.reservedTokens[i] == EMPTY) revert ZeroAddressNotAllowed();
        nativeToBridgedToken[_initializationData.sourceChainId][
          _initializationData.reservedTokens[i]
        ] = RESERVED_STATUS;
        emit TokenReserved(_initializationData.reservedTokens[i]);
        ++i;
      }
    }
  }

  /**
   * @notice Sets permissions for a list of addresses and their roles as well as initialises the PauseManager pauseType:role mappings.
   * @dev This function is a reinitializer and can only be called once per version. Should be called using an upgradeAndCall transaction to the ProxyAdmin.
   * @param _defaultAdmin The default admin account's address.
   * @param _roleAddresses The list of addresses and roles to assign permissions to.
   * @param _pauseTypeRoles The list of pause types to associate with roles.
   * @param _unpauseTypeRoles The list of unpause types to associate with roles.
   */
  function reinitializePauseTypesAndPermissions(
    address _defaultAdmin,
    RoleAddress[] calldata _roleAddresses,
    PauseTypeRole[] calldata _pauseTypeRoles,
    PauseTypeRole[] calldata _unpauseTypeRoles
  ) external reinitializer(2) {
    if (_defaultAdmin == address(0)) {
      revert ZeroAddressNotAllowed();
    }

    _grantRole(DEFAULT_ADMIN_ROLE, _defaultAdmin);

    assembly {
      /// @dev Wiping the storage slot 101 of _owner as it is replaced by AccessControl and there is now the ERC165 __gap in its place.
      sstore(101, 0)
      /// @dev Wiping the storage slot 213 of _status as it is replaced by ReentrancyGuardUpgradeable at slot 1.
      sstore(213, 0)
    }

    __ReentrancyGuard_init();
    __PauseManager_init(_pauseTypeRoles, _unpauseTypeRoles);
    __Permissions_init(_roleAddresses);
  }

  /**
   * @notice This function is the single entry point to bridge tokens to the
   *   other chain, both for native and already bridged tokens. You can use it
   *   to bridge any ERC-20. If the token is bridged for the first time an ERC-20
   *   (BridgedToken.sol) will be automatically deployed on the target chain.
   * @dev User should first allow the bridge to transfer tokens on his behalf.
   *   Alternatively, you can use BridgeTokenWithPermit to do so in a single
   *   transaction. If you want the transfer to be automatically executed on the
   *   destination chain. You should send enough ETH to pay the postman fees.
   *   Note that Linea can reserve some tokens (which use a dedicated bridge).
   *   In this case, the token cannot be bridged. Linea can only reserve tokens
   *   that have not been bridged yet.
   *   Linea can pause the bridge for security reason. In this case new bridge
   *   transaction would revert.
   * @dev Note: If, when bridging an unbridged token and decimals are unknown,
   * the call will revert to prevent mismatched decimals. Only those ERC-20s,
   * with a decimals function are supported.
   * @param _token The address of the token to be bridged.
   * @param _amount The amount of the token to be bridged.
   * @param _recipient The address that will receive the tokens on the other chain.
   */
  function bridgeToken(
    address _token,
    uint256 _amount,
    address _recipient
  ) public payable nonZeroAddress(_token) nonZeroAddress(_recipient) nonZeroAmount(_amount) nonReentrant {
    _requireTypeAndGeneralNotPaused(PauseType.INITIATE_TOKEN_BRIDGING);
    uint256 sourceChainIdCache = sourceChainId;
    address nativeMappingValue = nativeToBridgedToken[sourceChainIdCache][_token];
    if (nativeMappingValue == RESERVED_STATUS) {
      // Token is reserved
      revert ReservedToken(_token);
    }

    address nativeToken = bridgedToNativeToken[_token];
    uint256 chainId;
    bytes memory tokenMetadata;

    if (nativeToken != EMPTY) {
      BridgedToken(_token).burn(msg.sender, _amount);
      chainId = targetChainId;
    } else {
      // Token is native

      // For tokens with special fee logic, ensure that only the amount received
      // by the bridge will be minted on the target chain.
      uint256 balanceBefore = IERC20Upgradeable(_token).balanceOf(address(this));
      IERC20Upgradeable(_token).safeTransferFrom(msg.sender, address(this), _amount);
      _amount = IERC20Upgradeable(_token).balanceOf(address(this)) - balanceBefore;
      nativeToken = _token;

      if (nativeMappingValue == EMPTY) {
        // New token
        nativeToBridgedToken[sourceChainIdCache][_token] = NATIVE_STATUS;
        emit NewToken(_token);
      }

      // Send Metadata only when the token has not been deployed on the other chain yet
      if (nativeMappingValue != DEPLOYED_STATUS) {
        tokenMetadata = abi.encode(_safeName(_token), _safeSymbol(_token), _safeDecimals(_token));
      }
      chainId = sourceChainIdCache;
    }
    messageService.sendMessage{ value: msg.value }(
      remoteSender,
      msg.value, // fees
      abi.encodeCall(ITokenBridge.completeBridging, (nativeToken, _amount, _recipient, chainId, tokenMetadata))
    );
    emit BridgingInitiatedV2(msg.sender, _recipient, _token, _amount);
  }

  /**
   * @notice Similar to `bridgeToken` function but allows to pass additional
   *   permit data to do the ERC-20 approval in a single transaction.
   * @notice _permit can fail silently, don't rely on this function passing as a form
   *   of authentication
   * @dev There is no need for validation at this level as the validation on pausing,
   * and empty values exists on the "bridgeToken" call.
   * @param _token The address of the token to be bridged.
   * @param _amount The amount of the token to be bridged.
   * @param _recipient The address that will receive the tokens on the other chain.
   * @param _permitData The permit data for the token, if applicable.
   */
  function bridgeTokenWithPermit(
    address _token,
    uint256 _amount,
    address _recipient,
    bytes calldata _permitData
  ) external payable {
    if (_permitData.length != 0) {
      _permit(_token, _permitData);
    }
    bridgeToken(_token, _amount, _recipient);
  }

  /**
   * @dev It can only be called from the Message Service. To finalize the bridging
   *   process, a user or postman needs to use the `claimMessage` function of the
   *   Message Service to trigger the transaction.
   * @param _nativeToken The address of the token on its native chain.
   * @param _amount The amount of the token to be received.
   * @param _recipient The address that will receive the tokens.
   * @param _chainId The token's origin layer chainId
   * @param _tokenMetadata Additional data used to deploy the bridged token if it
   *   doesn't exist already.
   */
  function completeBridging(
    address _nativeToken,
    uint256 _amount,
    address _recipient,
    uint256 _chainId,
    bytes calldata _tokenMetadata
  )
    external
    nonReentrant
    onlyMessagingService
    onlyAuthorizedRemoteSender
    whenTypeAndGeneralNotPaused(PauseType.COMPLETE_TOKEN_BRIDGING)
  {
    address nativeMappingValue = nativeToBridgedToken[_chainId][_nativeToken];
    address bridgedToken;

    if (nativeMappingValue == NATIVE_STATUS || nativeMappingValue == DEPLOYED_STATUS) {
      // Token is native on the local chain
      IERC20Upgradeable(_nativeToken).safeTransfer(_recipient, _amount);
    } else {
      bridgedToken = nativeMappingValue;
      if (nativeMappingValue == EMPTY) {
        // New token
        bridgedToken = deployBridgedToken(_nativeToken, _tokenMetadata, sourceChainId);
        bridgedToNativeToken[bridgedToken] = _nativeToken;
        nativeToBridgedToken[targetChainId][_nativeToken] = bridgedToken;
      }
      BridgedToken(bridgedToken).mint(_recipient, _amount);
    }
    emit BridgingFinalizedV2(_nativeToken, bridgedToken, _amount, _recipient);
  }

  /**
   * @dev Change the address of the Message Service.
   * @dev SET_MESSAGE_SERVICE_ROLE is required to execute.
   * @param _messageService The address of the new Message Service.
   */
  function setMessageService(
    address _messageService
  ) external nonZeroAddress(_messageService) onlyRole(SET_MESSAGE_SERVICE_ROLE) {
    address oldMessageService = address(messageService);
    messageService = IMessageService(_messageService);
    emit MessageServiceUpdated(_messageService, oldMessageService, msg.sender);
  }

  /**
   * @dev Change the status to DEPLOYED to the tokens passed in parameter
   *    Will call the method setDeployed on the other chain using the message Service
   * @param _tokens Array of bridged tokens that have been deployed.
   */
  function confirmDeployment(address[] memory _tokens) external payable {
    uint256 tokensLength = _tokens.length;
    if (tokensLength == 0) {
      revert TokenListEmpty();
    }

    // Check that the tokens have actually been deployed
    for (uint256 i; i < tokensLength; i++) {
      address nativeToken = bridgedToNativeToken[_tokens[i]];
      if (nativeToken == EMPTY) {
        revert TokenNotDeployed(_tokens[i]);
      }
      _tokens[i] = nativeToken;
    }

    messageService.sendMessage{ value: msg.value }(
      remoteSender,
      msg.value, // fees
      abi.encodeCall(ITokenBridge.setDeployed, (_tokens))
    );

    emit DeploymentConfirmed(_tokens, msg.sender);
  }

  /**
   * @dev Change the status of tokens to DEPLOYED. New bridge transaction will not
   *   contain token metadata, which save gas.
   *   Can only be called from the Message Service. A user or postman needs to use
   *   the `claimMessage` function of the Message Service to trigger the transaction.
   * @param _nativeTokens Array of native tokens for which the DEPLOYED status must be set.
   */
  function setDeployed(address[] calldata _nativeTokens) external onlyMessagingService onlyAuthorizedRemoteSender {
    unchecked {
      uint256 cachedSourceChainId = sourceChainId;
      for (uint256 i; i < _nativeTokens.length; ) {
        nativeToBridgedToken[cachedSourceChainId][_nativeTokens[i]] = DEPLOYED_STATUS;
        emit TokenDeployed(_nativeTokens[i]);
        ++i;
      }
    }
  }

  /**
   * @dev Sets the address of the remote token bridge. Can only be called once.
   * @dev SET_REMOTE_TOKENBRIDGE_ROLE is required to execute.
   * @param _remoteTokenBridge The address of the remote token bridge to be set.
   */
  function setRemoteTokenBridge(address _remoteTokenBridge) external onlyRole(SET_REMOTE_TOKENBRIDGE_ROLE) {
    if (remoteSender != EMPTY) revert RemoteTokenBridgeAlreadySet(remoteSender);
    _setRemoteSender(_remoteTokenBridge);
    emit RemoteTokenBridgeSet(_remoteTokenBridge, msg.sender);
  }

  /**
   * @dev Deploy a new EC20 contract for bridged token using a beacon proxy pattern.
   *   To adapt to future requirements, Linea can update the implementation of
   *   all (existing and future) contracts by updating the beacon. This update is
   *   subject to a delay by a time lock.
   *   Contracts are deployed using CREATE2 so deployment address is deterministic.
   * @param _nativeToken The address of the native token on the source chain.
   * @param _tokenMetadata The encoded metadata for the token.
   * @param _chainId The chain id on which the token will be deployed, used to calculate the salt
   * @return bridgedTokenAddress The address of the newly deployed BridgedToken contract.
   */
  function deployBridgedToken(
    address _nativeToken,
    bytes calldata _tokenMetadata,
    uint256 _chainId
  ) internal returns (address bridgedTokenAddress) {
    bridgedTokenAddress = address(
      new BeaconProxy{ salt: EfficientLeftRightKeccak._efficientKeccak(_chainId, _nativeToken) }(tokenBeacon, "")
    );

    (string memory name, string memory symbol, uint8 decimals) = abi.decode(_tokenMetadata, (string, string, uint8));
    BridgedToken(bridgedTokenAddress).initialize(name, symbol, decimals);
    emit NewTokenDeployed(bridgedTokenAddress, _nativeToken);
  }

  /**
   * @dev Linea can reserve tokens. In this case, the token cannot be bridged.
   *   Linea can only reserve tokens that have not been bridged before.
   * @dev SET_RESERVED_TOKEN_ROLE is required to execute.
   * @notice Make sure that _token is native to the current chain
   *   where you are calling this function from
   * @param _token The address of the token to be set as reserved.
   */
  function setReserved(
    address _token
  ) external nonZeroAddress(_token) isNewToken(_token) onlyRole(SET_RESERVED_TOKEN_ROLE) {
    nativeToBridgedToken[sourceChainId][_token] = RESERVED_STATUS;
    emit TokenReserved(_token);
  }

  /**
   * @dev Removes a token from the reserved list.
   * @dev REMOVE_RESERVED_TOKEN_ROLE is required to execute.
   * @param _token The address of the token to be removed from the reserved list.
   */
  function removeReserved(address _token) external nonZeroAddress(_token) onlyRole(REMOVE_RESERVED_TOKEN_ROLE) {
    uint256 cachedSourceChainId = sourceChainId;

    if (nativeToBridgedToken[cachedSourceChainId][_token] != RESERVED_STATUS) revert NotReserved(_token);
    nativeToBridgedToken[cachedSourceChainId][_token] = EMPTY;

    emit ReservationRemoved(_token);
  }

  /**
   * @dev Linea can set a custom ERC-20 contract for specific ERC-20.
   *   For security purposes, Linea can only call this function if the token has
   *   not been bridged yet.
   * @dev SET_CUSTOM_CONTRACT_ROLE is required to execute.
   * @param _nativeToken The address of the token on the source chain.
   * @param _targetContract The address of the custom contract.
   */
  function setCustomContract(
    address _nativeToken,
    address _targetContract
  )
    external
    nonZeroAddress(_nativeToken)
    nonZeroAddress(_targetContract)
    onlyRole(SET_CUSTOM_CONTRACT_ROLE)
    isNewToken(_nativeToken)
  {
    if (bridgedToNativeToken[_targetContract] != EMPTY) {
      revert AlreadyBrigedToNativeTokenSet(_targetContract);
    }
    if (_targetContract == NATIVE_STATUS || _targetContract == DEPLOYED_STATUS || _targetContract == RESERVED_STATUS) {
      revert StatusAddressNotAllowed(_targetContract);
    }

    uint256 cachedTargetChainId = targetChainId;

    if (nativeToBridgedToken[cachedTargetChainId][_nativeToken] != EMPTY) {
      revert NativeToBridgedTokenAlreadySet(_nativeToken);
    }

    nativeToBridgedToken[cachedTargetChainId][_nativeToken] = _targetContract;
    bridgedToNativeToken[_targetContract] = _nativeToken;
    emit CustomContractSet(_nativeToken, _targetContract, msg.sender);
  }

  // Helpers to safely get the metadata from a token, inspired by
  // https://github.com/traderjoe-xyz/joe-core/blob/main/contracts/MasterChefJoeV3.sol#L55-L95

  /**
   * @dev Provides a safe ERC-20.name version which returns 'NO_NAME' as fallback string.
   * @param _token The address of the ERC-20 token contract
   * @return tokenName Returns the string of the token name.
   */
  function _safeName(address _token) internal view returns (string memory tokenName) {
    (bool success, bytes memory data) = _token.staticcall(METADATA_NAME);
    tokenName = success ? _returnDataToString(data) : "NO_NAME";
  }

  /**
   * @dev Provides a safe ERC-20.symbol version which returns 'NO_SYMBOL' as fallback string
   * @param _token The address of the ERC-20 token contract
   * @return symbol Returns the string of the symbol.
   */
  function _safeSymbol(address _token) internal view returns (string memory symbol) {
    (bool success, bytes memory data) = _token.staticcall(METADATA_SYMBOL);
    symbol = success ? _returnDataToString(data) : "NO_SYMBOL";
  }

  /**
   * @notice Provides a safe ERC-20.decimals version which reverts when decimals are unknown
   *   Note Tokens with (decimals > 255) are not supported
   * @param _token The address of the ERC-20 token contract
   * @return Returns the token's decimals value.
   */
  function _safeDecimals(address _token) internal view returns (uint8) {
    (bool success, bytes memory data) = _token.staticcall(METADATA_DECIMALS);

    if (success && data.length == VALID_DECIMALS_ENCODING_LENGTH) {
      return abi.decode(data, (uint8));
    }

    revert DecimalsAreUnknown(_token);
  }

  /**
   * @dev Converts returned data to string. Returns 'NOT_VALID_ENCODING' as fallback value.
   * @param _data returned data.
   * @return decodedString The decoded string data.
   */
  function _returnDataToString(bytes memory _data) internal pure returns (string memory decodedString) {
    if (_data.length >= MINIMUM_STRING_ABI_DECODE_LENGTH) {
      return abi.decode(_data, (string));
    } else if (_data.length != SHORT_STRING_ENCODING_LENGTH) {
      return "NOT_VALID_ENCODING";
    }

    // Since the strings on bytes32 are encoded left-right, check the first zero in the data
    uint256 nonZeroBytes;
    unchecked {
      while (nonZeroBytes < SHORT_STRING_ENCODING_LENGTH && _data[nonZeroBytes] != 0) {
        nonZeroBytes++;
      }
    }

    // If the first one is 0, we do not handle the encoding
    if (nonZeroBytes == 0) {
      return "NOT_VALID_ENCODING";
    }
    // Create a byte array with nonZeroBytes length
    bytes memory bytesArray = new bytes(nonZeroBytes);
    unchecked {
      for (uint256 i; i < nonZeroBytes; ) {
        bytesArray[i] = _data[i];
        ++i;
      }
    }
    decodedString = string(bytesArray);
  }

  /**
   * @notice Call the token permit method of extended ERC-20
   * @notice Only support tokens implementing ERC-2612
   * @param _token ERC-20 token address
   * @param _permitData Raw data of the call `permit` of the token
   */
  function _permit(address _token, bytes calldata _permitData) internal {
    if (bytes4(_permitData[:4]) != _PERMIT_SELECTOR)
      revert InvalidPermitData(bytes4(_permitData[:4]), _PERMIT_SELECTOR);
    // Decode the permit data
    // The parameters are:
    // 1. owner: The address of the wallet holding the tokens
    // 2. spender: The address of the entity permitted to spend the tokens
    // 3. value: The maximum amount of tokens the spender is allowed to spend
    // 4. deadline: The time until which the permit is valid
    // 5. v: Part of the signature (along with r and s), these three values form the signature of the permit
    // 6. r: Part of the signature
    // 7. s: Part of the signature
    (address owner, address spender, uint256 amount, uint256 deadline, uint8 v, bytes32 r, bytes32 s) = abi.decode(
      _permitData[4:],
      (address, address, uint256, uint256, uint8, bytes32, bytes32)
    );
    if (owner != msg.sender) revert PermitNotFromSender(owner);
    if (spender != address(this)) revert PermitNotAllowingBridge(spender);

    if (IERC20Upgradeable(_token).allowance(owner, spender) < amount) {
      IERC20PermitUpgradeable(_token).permit(msg.sender, address(this), amount, deadline, v, r, s);
    }
  }
}
