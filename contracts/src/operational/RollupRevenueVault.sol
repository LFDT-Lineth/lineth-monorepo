// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.30;

import { AccessControlUpgradeable } from "@openzeppelin/contracts-upgradeable/access/AccessControlUpgradeable.sol";
import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { L2MessageService } from "../messaging/l2/L2MessageService.sol";
import { TokenBridge } from "../bridging/token/TokenBridge.sol";
import { IRollupRevenueVault } from "./interfaces/IRollupRevenueVault.sol";

/**
 * @title Upgradeable Rollup Revenue Vault Contract.
 * @notice Accepts rollup revenue, and performs burning operations.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract RollupRevenueVault is AccessControlUpgradeable, IRollupRevenueVault {
  bytes32 public constant INVOICE_SUBMITTER_ROLE = keccak256("INVOICE_SUBMITTER_ROLE");
  bytes32 public constant BURNER_ROLE = keccak256("BURNER_ROLE");

  /// @notice Percentage of ETH to be burnt when performing the burn and bridge operation.
  uint256 public constant ETH_BURNT_PERCENTAGE = 20;

  /// @notice Decentralized exchange contract for swapping ETH to LINEA tokens.
  address public dex;
  /// @notice Address to receive invoice payments.
  address public invoicePaymentReceiver;
  /// @notice Amount of invoice arrears.
  uint256 public invoiceArrears;
  /// @notice Timestamp of the last invoice.
  uint256 public lastInvoiceDate;
  /// @notice Address of the token bridge contract.
  TokenBridge public tokenBridge;
  /// @notice Address of the L2 message service contract.
  L2MessageService public messageService;
  /// @notice Address of the L1 LINEA token burner contract to which LINEA tokens are bridged for burning.
  address public l1LineaTokenBurner;
  /// @notice Address of the LINEA token contract.
  address public lineaToken;

  /// @custom:oz-upgrades-unsafe-allow constructor
  constructor() {
    _disableInitializers();
  }

  /**
   * @notice Initializes the contract state.
   * @param _lastInvoiceDate Timestamp of the last invoice.
   * @param _defaultAdmin Address to be granted the default admin role.
   * @param _invoiceSubmitter Address to be granted the invoice submitter role.
   * @param _burner Address to be granted the burner role.
   * @param _invoicePaymentReceiver Address to receive invoice payments.
   * @param _tokenBridge Address of the token bridge contract.
   * @param _messageService Address of the L2 message service contract.
   * @param _l1LineaTokenBurner Address of the L1 LINEA token burner contract.
   * @param _lineaToken Address of the LINEA token contract.
   * @param _dex Address of the DEX contract.
   */
  function initialize(
    uint256 _lastInvoiceDate,
    address _defaultAdmin,
    address _invoiceSubmitter,
    address _burner,
    address _invoicePaymentReceiver,
    address _tokenBridge,
    address _messageService,
    address _l1LineaTokenBurner,
    address _lineaToken,
    address _dex
  ) external initializer {
    __AccessControl_init();
    __RollupRevenueVault_init(
      _lastInvoiceDate,
      _defaultAdmin,
      _invoiceSubmitter,
      _burner,
      _invoicePaymentReceiver,
      _tokenBridge,
      _messageService,
      _l1LineaTokenBurner,
      _lineaToken,
      _dex
    );
  }

  /**
   * @notice Reinitializes the contract state for upgrade.
   * @param _lastInvoiceDate Timestamp of the last invoice.
   * @param _defaultAdmin Address to be granted the default admin role.
   * @param _invoiceSubmitter Address to be granted the invoice submitter role.
   * @param _burner Address to be granted the burner role.
   * @param _invoicePaymentReceiver Address to receive invoice payments.
   * @param _tokenBridge Address of the token bridge contract.
   * @param _messageService Address of the L2 message service contract.
   * @param _l1LineaTokenBurner Address of the L1 LINEA token burner contract.
   * @param _lineaToken Address of the LINEA token contract.
   * @param _dex Address of the DEX contract.
   */
  function initializeRolesAndStorageVariables(
    uint256 _lastInvoiceDate,
    address _defaultAdmin,
    address _invoiceSubmitter,
    address _burner,
    address _invoicePaymentReceiver,
    address _tokenBridge,
    address _messageService,
    address _l1LineaTokenBurner,
    address _lineaToken,
    address _dex
  ) external reinitializer(2) {
    __AccessControl_init();
    __RollupRevenueVault_init(
      _lastInvoiceDate,
      _defaultAdmin,
      _invoiceSubmitter,
      _burner,
      _invoicePaymentReceiver,
      _tokenBridge,
      _messageService,
      _l1LineaTokenBurner,
      _lineaToken,
      _dex
    );
  }

  function __RollupRevenueVault_init(
    uint256 _lastInvoiceDate,
    address _defaultAdmin,
    address _invoiceSubmitter,
    address _burner,
    address _invoicePaymentReceiver,
    address _tokenBridge,
    address _messageService,
    address _l1LineaTokenBurner,
    address _lineaToken,
    address _dex
  ) internal onlyInitializing {
    require(_lastInvoiceDate != 0, ZeroTimestampNotAllowed());
    require(_defaultAdmin != address(0), ZeroAddressNotAllowed());
    require(_invoiceSubmitter != address(0), ZeroAddressNotAllowed());
    require(_burner != address(0), ZeroAddressNotAllowed());
    require(_invoicePaymentReceiver != address(0), ZeroAddressNotAllowed());
    require(_tokenBridge != address(0), ZeroAddressNotAllowed());
    require(_messageService != address(0), ZeroAddressNotAllowed());
    require(_l1LineaTokenBurner != address(0), ZeroAddressNotAllowed());
    require(_lineaToken != address(0), ZeroAddressNotAllowed());
    require(_dex != address(0), ZeroAddressNotAllowed());

    _grantRole(DEFAULT_ADMIN_ROLE, _defaultAdmin);
    _grantRole(INVOICE_SUBMITTER_ROLE, _invoiceSubmitter);
    _grantRole(BURNER_ROLE, _burner);

    lastInvoiceDate = _lastInvoiceDate;

    invoicePaymentReceiver = _invoicePaymentReceiver;
    tokenBridge = TokenBridge(_tokenBridge);
    messageService = L2MessageService(_messageService);
    l1LineaTokenBurner = _l1LineaTokenBurner;
    lineaToken = _lineaToken;
    dex = _dex;
  }

  /**
   * @notice Submit invoice to pay to the designated receiver.
   * @param _startTimestamp Start of the period the invoice is covering.
   * @param _endTimestamp End of the period the invoice is covering.
   * @param _invoiceAmount New invoice amount.
   */
  function submitInvoice(
    uint256 _startTimestamp,
    uint256 _endTimestamp,
    uint256 _invoiceAmount
  ) external payable onlyRole(INVOICE_SUBMITTER_ROLE) {
    require(_startTimestamp == lastInvoiceDate + 1, TimestampsNotInSequence());
    require(_endTimestamp > _startTimestamp, EndTimestampMustBeGreaterThanStartTimestamp());
    require(_invoiceAmount != 0, ZeroInvoiceAmount());

    address payable receiver = payable(invoicePaymentReceiver);
    uint256 balanceAvailable = address(this).balance;

    uint256 totalAmountOwing = invoiceArrears + _invoiceAmount;
    uint256 amountToPay = (balanceAvailable < totalAmountOwing) ? balanceAvailable : totalAmountOwing;

    invoiceArrears = totalAmountOwing - amountToPay;
    lastInvoiceDate = _endTimestamp;

    if (amountToPay > 0) {
      (bool success, ) = receiver.call{ value: amountToPay }("");
      require(success, InvoiceTransferFailed());
    }

    emit InvoiceProcessed(receiver, _startTimestamp, _endTimestamp, amountToPay, _invoiceAmount);
  }

  /**
   * @notice Burns 20% of the ETH balance and uses the rest to buy LINEA tokens which are then bridged to L1 to be burned.
   * @param _swapData Encoded calldata for the DEX swap function.
   */
  function burnAndBridge(bytes calldata _swapData) external onlyRole(BURNER_ROLE) {
    require(invoiceArrears == 0, InvoiceInArrears());

    uint256 minimumFee = messageService.minimumFeeInWei();

    require(address(this).balance > minimumFee, InsufficientBalance());

    uint256 balanceAvailable = address(this).balance - minimumFee;

    uint256 ethToBurn = (balanceAvailable * ETH_BURNT_PERCENTAGE) / 100;
    (bool success, ) = address(0).call{ value: ethToBurn }("");
    require(success, EthBurnFailed());

    (bool swapSuccess, bytes memory returnData) = dex.call{ value: balanceAvailable - ethToBurn }(_swapData);
    require(swapSuccess, DexSwapFailed());

    uint256 numLineaTokens = abi.decode(returnData, (uint256));
    require(numLineaTokens > 0, ZeroLineaTokensReceived());

    IERC20(lineaToken).approve(address(tokenBridge), numLineaTokens);

    tokenBridge.bridgeToken{ value: minimumFee }(lineaToken, numLineaTokens, l1LineaTokenBurner);

    emit EthBurntSwappedAndBridged(ethToBurn, numLineaTokens);
  }

  /**
   * @notice Update the invoice payment receiver.
   * @param _newInvoicePaymentReceiver New invoice payment receiver address.
   */
  function updateInvoicePaymentReceiver(address _newInvoicePaymentReceiver) external onlyRole(DEFAULT_ADMIN_ROLE) {
    require(_newInvoicePaymentReceiver != address(0), ZeroAddressNotAllowed());

    address currentInvoicePaymentReceiver = invoicePaymentReceiver;
    require(_newInvoicePaymentReceiver != currentInvoicePaymentReceiver, ExistingAddressTheSame());

    invoicePaymentReceiver = _newInvoicePaymentReceiver;
    emit InvoicePaymentReceiverUpdated(currentInvoicePaymentReceiver, _newInvoicePaymentReceiver);
  }

  /**
   * @notice Update the invoice arrears.
   * @param _newInvoiceArrears New invoice arrears value.
   * @param _lastInvoiceDate Timestamp of the last invoice.
   */
  function updateInvoiceArrears(
    uint256 _newInvoiceArrears,
    uint256 _lastInvoiceDate
  ) external onlyRole(DEFAULT_ADMIN_ROLE) {
    require(_lastInvoiceDate >= lastInvoiceDate, InvoiceDateTooOld());

    invoiceArrears = _newInvoiceArrears;
    lastInvoiceDate = _lastInvoiceDate;
    emit InvoiceArrearsUpdated(_newInvoiceArrears, _lastInvoiceDate);
  }

  /**
   * @notice Updates the address of the L1 LINEA token burner contract.
   * @param _newL1LineaTokenBurner Address of the new L1 LINEA token burner contract.
   */
  function updateL1LineaTokenBurner(address _newL1LineaTokenBurner) external onlyRole(DEFAULT_ADMIN_ROLE) {
    require(_newL1LineaTokenBurner != address(0), ZeroAddressNotAllowed());

    address currentL1LineaTokenBurner = l1LineaTokenBurner;
    require(_newL1LineaTokenBurner != currentL1LineaTokenBurner, ExistingAddressTheSame());

    l1LineaTokenBurner = _newL1LineaTokenBurner;
    emit L1LineaTokenBurnerUpdated(currentL1LineaTokenBurner, _newL1LineaTokenBurner);
  }

  /**
   * @notice Updates the address of the DEX contract.
   * @param _newDex Address of the new DEX contract.
   */
  function updateDex(address _newDex) external onlyRole(DEFAULT_ADMIN_ROLE) {
    require(_newDex != address(0), ZeroAddressNotAllowed());

    address currentDex = dex;
    require(_newDex != currentDex, ExistingAddressTheSame());

    dex = _newDex;
    emit DexUpdated(currentDex, _newDex);
  }

  /**
   * @notice Fallback function - Receives Funds.
   */
  fallback() external payable {
    emit EthReceived(msg.value);
  }

  /**
   * @notice Receive function - Receives Funds.
   */
  receive() external payable {
    emit EthReceived(msg.value);
  }
}
