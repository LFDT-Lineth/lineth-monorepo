// SPDX-License-Identifier: Apache-2.0 OR MIT
pragma solidity 0.8.33;

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

  /// @notice Decentralized exchange adapter contract for swapping ETH to LINEA tokens.
  address public dexSwapAdapter;
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
   * @notice Reinitializes the contract state for upgrade.
   * @param _lastInvoiceDate The default or starting timestamp for invoices less 1 second.
   * @param _defaultAdmin Address to be granted the default admin role.
   * @param _invoiceSubmitter Address to be granted the invoice submitter role.
   * @param _burner Address to be granted the burner role.
   * @param _invoicePaymentReceiver Address to receive invoice payments.
   * @param _tokenBridge Address of the token bridge contract.
   * @param _messageService Address of the L2 message service contract.
   * @param _l1LineaTokenBurner Address of the L1 LINEA token burner contract.
   * @param _lineaToken Address of the LINEA token contract.
   * @param _dexSwapAdapter Address of the DEX swap adapter contract.
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
    address _dexSwapAdapter
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
      _dexSwapAdapter
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
    address _dexSwapAdapter
  ) internal onlyInitializing {
    require(_lastInvoiceDate != 0, ZeroTimestampNotAllowed());
    require(_lastInvoiceDate < block.timestamp, FutureInvoicesNotAllowed());

    require(_defaultAdmin != address(0), ZeroAddressNotAllowed());
    require(_invoiceSubmitter != address(0), ZeroAddressNotAllowed());
    require(_burner != address(0), ZeroAddressNotAllowed());
    require(_invoicePaymentReceiver != address(0), ZeroAddressNotAllowed());
    require(_tokenBridge != address(0), ZeroAddressNotAllowed());
    require(_messageService != address(0), ZeroAddressNotAllowed());
    require(_l1LineaTokenBurner != address(0), ZeroAddressNotAllowed());
    require(_lineaToken != address(0), ZeroAddressNotAllowed());
    require(_dexSwapAdapter != address(0), ZeroAddressNotAllowed());

    _grantRole(DEFAULT_ADMIN_ROLE, _defaultAdmin);
    _grantRole(INVOICE_SUBMITTER_ROLE, _invoiceSubmitter);
    _grantRole(BURNER_ROLE, _burner);

    lastInvoiceDate = _lastInvoiceDate;

    invoicePaymentReceiver = _invoicePaymentReceiver;
    tokenBridge = TokenBridge(_tokenBridge);
    messageService = L2MessageService(_messageService);
    l1LineaTokenBurner = _l1LineaTokenBurner;
    lineaToken = _lineaToken;
    dexSwapAdapter = _dexSwapAdapter;

    emit RollupRevenueVaultInitialized(
      _lastInvoiceDate,
      _invoicePaymentReceiver,
      _tokenBridge,
      _messageService,
      _l1LineaTokenBurner,
      _lineaToken,
      _dexSwapAdapter
    );
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
    require(_endTimestamp < block.timestamp, FutureInvoicesNotAllowed());
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
    _payArrears();

    uint256 minimumFee = messageService.minimumFeeInWei();

    if (address(this).balance > minimumFee) {
      uint256 balanceAvailable = address(this).balance - minimumFee;

      uint256 ethToBurn = (balanceAvailable * ETH_BURNT_PERCENTAGE) / 100;
      (bool success, ) = address(0).call{ value: ethToBurn }("");
      require(success, EthBurnFailed());

      (bool swapSuccess, ) = dexSwapAdapter.call{ value: balanceAvailable - ethToBurn }(_swapData);
      require(swapSuccess, DexSwapFailed());

      address lineaTokenAddress = lineaToken;
      TokenBridge tokenBridgeContract = tokenBridge;

      uint256 lineaTokenBalanceAfter = IERC20(lineaTokenAddress).balanceOf(address(this));

      IERC20(lineaTokenAddress).approve(address(tokenBridgeContract), lineaTokenBalanceAfter);

      tokenBridgeContract.bridgeToken{ value: minimumFee }(
        lineaTokenAddress,
        lineaTokenBalanceAfter,
        l1LineaTokenBurner
      );

      emit EthBurntSwappedAndBridged(ethToBurn, lineaTokenBalanceAfter);
    }
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
   * @param _invoiceArrears New invoice arrears value.
   * @param _lastInvoiceDate Timestamp of the last invoice.
   */
  function updateInvoiceArrears(
    uint256 _invoiceArrears,
    uint256 _lastInvoiceDate
  ) external onlyRole(DEFAULT_ADMIN_ROLE) {
    require(_lastInvoiceDate >= lastInvoiceDate, InvoiceDateTooOld());
    require(_lastInvoiceDate < block.timestamp, FutureInvoicesNotAllowed());

    emit InvoiceArrearsUpdated(invoiceArrears, _invoiceArrears, lastInvoiceDate, _lastInvoiceDate);

    invoiceArrears = _invoiceArrears;
    lastInvoiceDate = _lastInvoiceDate;
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
   * @notice Updates the address of the DEX swap adapter contract.
   * @param _newDexSwapAdapter Address of the new DEX swap adapter contract.
   */
  function updateDexSwapAdapter(address _newDexSwapAdapter) external onlyRole(DEFAULT_ADMIN_ROLE) {
    require(_newDexSwapAdapter != address(0), ZeroAddressNotAllowed());

    address currentDexSwapAdapter = dexSwapAdapter;
    require(_newDexSwapAdapter != currentDexSwapAdapter, ExistingAddressTheSame());

    dexSwapAdapter = _newDexSwapAdapter;
    emit DexSwapAdapterUpdated(currentDexSwapAdapter, _newDexSwapAdapter);
  }

  /**
   * @notice Fallback function - Receives Funds.
   */
  fallback() external payable {
    require(msg.value > 0, NoEthSent());
    emit EthReceived(msg.value);
  }

  /**
   * @notice Receive function - Receives Funds.
   */
  receive() external payable {
    require(msg.value > 0, NoEthSent());
    emit EthReceived(msg.value);
  }

  /**
   * @notice Pays off arrears where applicable and balance permits.
   */
  function _payArrears() internal {
    uint256 balanceAvailable = address(this).balance;

    uint256 totalAmountOwing = invoiceArrears;
    uint256 amountToPay = (balanceAvailable < totalAmountOwing) ? balanceAvailable : totalAmountOwing;

    if (amountToPay > 0) {
      uint256 remainingArrears = totalAmountOwing - amountToPay;
      invoiceArrears = remainingArrears;

      (bool success, ) = payable(invoicePaymentReceiver).call{ value: amountToPay }("");
      require(success, InvoiceTransferFailed());
      emit ArrearsPaid(amountToPay, remainingArrears);
    }
  }
}
