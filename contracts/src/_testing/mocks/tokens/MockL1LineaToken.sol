// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

import { ERC20Upgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/ERC20Upgradeable.sol";
import { ERC20BurnableUpgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/extensions/ERC20BurnableUpgradeable.sol";
import { ERC20PermitUpgradeable } from "@openzeppelin/contracts-upgradeable/token/ERC20/extensions/ERC20PermitUpgradeable.sol";

interface IMockMessageService {
  /**
   * @notice Sends a message for transporting from the given chain.
   * @dev This function should be called with a msg.value = _value + _fee. The fee will be paid on the destination chain.
   * @param _to The destination address on the destination chain.
   * @param _fee The message service fee on the origin chain.
   * @param _calldata The calldata used by the destination message service to call the destination contract.
   */
  function sendMessage(address _to, uint256 _fee, bytes calldata _calldata) external payable;
}

interface IL2LineaToken {
  /**
   * @notice Synchronizes the total supply of the L1 Linea token from L1 Ethereum.
   * @dev NB: This function can only be called by the Linea Message Service.
   * @dev NB: This function must have originated from the Linea token on L1 Ethereum.
   * @param _l1LineaTokenTotalSupplySyncTime The L1 block.timestamp when the Linea token on L1 total supply was computed.
   * @param _l1LineaTokenSupply The total supply of the L1 Linea token.
   */
  function syncTotalSupplyFromL1(uint256 _l1LineaTokenTotalSupplySyncTime, uint256 _l1LineaTokenSupply) external;
}

/**
 * @title Contract to manage the Linea Token.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract MockL1LineaToken is ERC20Upgradeable, ERC20BurnableUpgradeable, ERC20PermitUpgradeable {
  /// @notice The role required to mint tokens.
  bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");

  /// @notice L1 message service contract address.
  address public l1MessageService;
  /// @notice L2 Linea token contract address.
  address public l2TokenAddress;

  /// @dev Thrown when an address is the zero address.
  error ZeroAddressNotAllowed();

  /**
   * @notice Emitted when the L1 total supply synchronization starts.
   * @param l1TotalSupply The total supply of the L1 token.
   */
  event L1TotalSupplySyncStarted(uint256 l1TotalSupply);

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes the contract.
   * @dev The default decimals of 18 applies to this token.
   * @param _l1MessageService The address of the L1 message service.
   * @param _l2TokenAddress The address of the L2 Linea token.
   * @param _tokenName The name of the token.
   * @param _tokenSymbol The symbol of the token.
   */
  function initialize(
    address _l1MessageService,
    address _l2TokenAddress,
    string calldata _tokenName,
    string calldata _tokenSymbol
  ) external initializer {
    __ERC20_init(_tokenName, _tokenSymbol);
    __ERC20Burnable_init();
    __ERC20Permit_init(_tokenName);

    l1MessageService = _l1MessageService;
    l2TokenAddress = _l2TokenAddress;
  }

  /**
   * @notice Mints the Linea token.
   * @dev NB: Only those with MINTER_ROLE can call this function.
   * @param _account Account being minted for.
   * @param _amount The amount being minted for the account.
   */
  function mint(address _account, uint256 _amount) external {
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
    IMockMessageService(l1MessageService).sendMessage(
      l2TokenAddress,
      0,
      abi.encodeCall(IL2LineaToken.syncTotalSupplyFromL1, (block.timestamp, totalSupply))
    );

    emit L1TotalSupplySyncStarted(totalSupply);
  }
}
