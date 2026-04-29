// SPDX-License-Identifier: Apache-2.0
pragma solidity ^0.8.33;

import { IPauseManager } from "../../../security/pausing/interfaces/IPauseManager.sol";
import { IPermissionsManager } from "../../../security/access/interfaces/IPermissionsManager.sol";

/**
 * @title Interface declaring Canonical Token Bridge struct, functions, events and errors.
 * @author ConsenSys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
interface ITokenBridge {
  /**
   * @dev Contract will be used as proxy implementation.
   * @param defaultAdmin The account to be given DEFAULT_ADMIN_ROLE on initialization.
   * @param messageService The address of the MessageService contract.
   * @param tokenBeacon The address of the tokenBeacon.
   * @param sourceChainId The source chain id of the current layer.
   * @param targetChainId The target chaind id of the targeted layer.
   * @param remoteSender Address of the remote token bridge.
   * @param reservedTokens The list of reserved tokens to be set.
   * @param roleAddresses The list of addresses and roles to assign permissions to.
   * @param pauseTypeRoles The list of pause types to associate with roles.
   * @param unpauseTypeRoles The list of unpause types to associate with roles.
   */
  struct InitializationData {
    address defaultAdmin;
    address messageService;
    address tokenBeacon;
    uint256 sourceChainId;
    uint256 targetChainId;
    address remoteSender;
    address[] reservedTokens;
    IPermissionsManager.RoleAddress[] roleAddresses;
    IPauseManager.PauseTypeRole[] pauseTypeRoles;
    IPauseManager.PauseTypeRole[] unpauseTypeRoles;
  }

  /**
   * @notice Emitted when the token bridge is initialized.
   * @param contractVersion The contract version.
   * @param initializationData The initialization data.
   */
  event TokenBridgeBaseInitialized(bytes8 indexed contractVersion, InitializationData initializationData);

  /**
   * @notice Emitted when the token address is reserved.
   * @param token The indexed token address.
   */
  event TokenReserved(address indexed token);

  /**
   * @notice Emitted when the token address reservation is removed.
   * @param token The indexed token address.
   */
  event ReservationRemoved(address indexed token);

  /**
   * @notice Emitted when the custom token address is set.
   * @param nativeToken The indexed nativeToken token address.
   * @param customContract The indexed custom contract address.
   * @param setBy The indexed address of who set the custom contract.
   */
  event CustomContractSet(address indexed nativeToken, address indexed customContract, address indexed setBy);

  /**
   * @notice Emitted when token bridging is initiated.
   * @dev DEPRECATED in favor of BridgingInitiatedV2.
   * @param sender The indexed sender address.
   * @param recipient The recipient address.
   * @param token The indexed token address.
   * @param amount The indexed token amount.
   */
  event BridgingInitiated(address indexed sender, address recipient, address indexed token, uint256 indexed amount);

  /**
   * @notice Emitted when token bridging is initiated.
   * @param sender The indexed sender address.
   * @param recipient The indexed recipient address.
   * @param token The indexed token address.
   * @param amount The token amount.
   */
  event BridgingInitiatedV2(address indexed sender, address indexed recipient, address indexed token, uint256 amount);

  /**
   * @notice Emitted when token bridging is finalized.
   * @dev DEPRECATED in favor of BridgingFinalizedV2.
   * @param nativeToken The indexed native token address.
   * @param bridgedToken The indexed bridged token address.
   * @param amount The indexed token amount.
   * @param recipient The recipient address.
   */
  event BridgingFinalized(
    address indexed nativeToken,
    address indexed bridgedToken,
    uint256 indexed amount,
    address recipient
  );

  /**
   * @notice Emitted when token bridging is finalized.
   * @param nativeToken The indexed native token address.
   * @param bridgedToken The indexed bridged token address.
   * @param amount The token amount.
   * @param recipient The indexed recipient address.
   */
  event BridgingFinalizedV2(
    address indexed nativeToken,
    address indexed bridgedToken,
    uint256 amount,
    address indexed recipient
  );

  /**
   * @notice Emitted when a new token is seen being bridged on the origin chain for the first time.
   * @param token The indexed token address.
   */
  event NewToken(address indexed token);

  /**
   * @notice Emitted when a new token is deployed.
   * @param bridgedToken The indexed bridged token address.
   * @param nativeToken The indexed native token address.
   */
  event NewTokenDeployed(address indexed bridgedToken, address indexed nativeToken);

  /**
   * @notice Emitted when the token is set as deployed.
   * @dev This can be triggered by anyone calling confirmDeployment on the alternate chain.
   * @param token The indexed token address.
   */
  event TokenDeployed(address indexed token);

  /**
   * @notice Emitted when the token deployment is confirmed.
   * @dev This can be triggered by anyone provided there is correctly mapped token data.
   * @param tokens The token address list.
   * @param confirmedBy The indexed address confirming deployment.
   */
  event DeploymentConfirmed(address[] tokens, address indexed confirmedBy);

  /**
   * @notice Emitted when the message service address is set.
   * @param newMessageService The indexed new message service address.
   * @param oldMessageService The indexed old message service address.
   * @param setBy The indexed address setting the new message service address.
   */
  event MessageServiceUpdated(
    address indexed newMessageService,
    address indexed oldMessageService,
    address indexed setBy
  );

  /**
   * @dev Thrown when attempting to bridge a reserved token.
   */
  error ReservedToken(address token);

  /**
   * @dev Thrown when attempting to reserve an already bridged token.
   */
  error AlreadyBridgedToken(address token);

  /**
   * @dev Thrown when the permit data is invalid.
   */
  error InvalidPermitData(bytes4 permitData, bytes4 permitSelector);

  /**
   * @dev Thrown when the permit is not from the sender.
   */
  error PermitNotFromSender(address owner);

  /**
   * @dev Thrown when the permit does not grant spending to the bridge.
   */
  error PermitNotAllowingBridge(address spender);

  /**
   * @dev Thrown when the amount being bridged is zero.
   */
  error ZeroAmountNotAllowed(uint256 amount);

  /**
   * @dev Thrown when trying to unreserve a non-reserved token.
   */
  error NotReserved(address token);

  /**
   * @dev Thrown when trying to confirm deployment of a non-deployed token.
   */
  error TokenNotDeployed(address token);

  /**
   * @dev Thrown when trying to set a custom contract on a bridged token.
   */
  error AlreadyBrigedToNativeTokenSet(address token);

  /**
   * @dev Thrown when trying to set a custom contract on an already set token.
   */
  error NativeToBridgedTokenAlreadySet(address token);

  /**
   * @dev Thrown when trying to set a token that is already either native, deployed or reserved.
   */
  error StatusAddressNotAllowed(address token);

  /**
   * @dev Thrown when the decimals for a token cannot be determined.
   */
  error DecimalsAreUnknown(address token);

  /**
   * @dev Thrown when the token list is empty.
   */
  error TokenListEmpty();

  /**
   * @dev Thrown when a chainId provided during initialization is zero.
   */
  error ZeroChainIdNotAllowed();

  /**
   * @dev Thrown when sourceChainId is the same as targetChainId during initialization.
   */
  error SourceChainSameAsTargetChain();

  /**
   * @notice Returns the ABI version and not the reinitialize version.
   * @return contractVersion The contract ABI version.
   */
  function CONTRACT_VERSION() external view returns (string memory contractVersion);

  /**
   * @notice Similar to `bridgeToken` function but allows to pass additional
   *   permit data to do the ERC-20 approval in a single transaction.
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
  ) external payable;

  /**
   * @dev It can only be called from the Message Service. To finalize the bridging
   *   process, a user or postman needs to use the `claimMessage` function of the
   *   Message Service to trigger the transaction.
   * @param _nativeToken The address of the token on its native chain.
   * @param _amount The amount of the token to be received.
   * @param _recipient The address that will receive the tokens.
   * @param _chainId The source chainId or target chainId for this token
   * @param _tokenMetadata Additional data used to deploy the bridged token if it
   *   doesn't exist already.
   */
  function completeBridging(
    address _nativeToken,
    uint256 _amount,
    address _recipient,
    uint256 _chainId,
    bytes calldata _tokenMetadata
  ) external;

  /**
   * @dev Change the status to DEPLOYED to the tokens passed in parameter
   *    Will call the method setDeployed on the other chain using the message Service
   * @param _tokens Array of bridged tokens that have been deployed.
   */
  function confirmDeployment(address[] memory _tokens) external payable;

  /**
   * @dev Change the address of the Message Service.
   * @param _messageService The address of the new Message Service.
   */
  function setMessageService(address _messageService) external;

  /**
   * @dev It can only be called from the Message Service. To change the status of
   *   the native tokens to DEPLOYED meaning they have been deployed on the other chain
   *   a user or postman needs to use the `claimMessage` function of the
   *   Message Service to trigger the transaction.
   * @param _nativeTokens The addresses of the native tokens.
   */
  function setDeployed(address[] memory _nativeTokens) external;

  /**
   * @dev Linea can reserve tokens. In this case, the token cannot be bridged.
   *   Linea can only reserve tokens that have not been bridged before.
   * @notice Make sure that _token is native to the current chain
   *   where you are calling this function from
   * @param _token The address of the token to be set as reserved.
   */
  function setReserved(address _token) external;

  /**
   * @dev Removes a token from the reserved list.
   * @param _token The address of the token to be removed from the reserved list.
   */
  function removeReserved(address _token) external;

  /**
   * @dev Linea can set a custom ERC-20 contract for specific ERC-20.
   *   For security purposes, Linea can only call this function if the token has
   *   not been bridged yet.
   * @param _nativeToken address of the token on the source chain.
   * @param _targetContract address of the custom contract.
   */
  function setCustomContract(address _nativeToken, address _targetContract) external;
}
