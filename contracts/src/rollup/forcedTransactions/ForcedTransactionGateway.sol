// SPDX-License-Identifier: AGPL-3.0
pragma solidity 0.8.33;
import { IGenericErrors } from "../../interfaces/IGenericErrors.sol";
import { IAcceptForcedTransactions } from "./interfaces/IAcceptForcedTransactions.sol";
import { IForcedTransactionGateway } from "./interfaces/IForcedTransactionGateway.sol";
import { IAddressFilter } from "./interfaces/IAddressFilter.sol";
import { Mimc } from "../../libraries/Mimc.sol";
import { FinalizedStateHashing } from "../../libraries/FinalizedStateHashing.sol";
import { AccessControl } from "@openzeppelin/contracts/access/AccessControl.sol";

import { LibRLP } from "solady/src/utils/LibRLP.sol";

import { ECDSA } from "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
/**
 * @title Contract to manage forced transactions on L1.
 * @author Consensys Software Inc.
 * @custom:security-contact security-report@linea.build
 */
contract ForcedTransactionGateway is AccessControl, IForcedTransactionGateway {
  using Mimc for *;
  using LibRLP for *;
  using FinalizedStateHashing for *;

  /// @notice Contains the destination address to store the forced transactions on.
  IAcceptForcedTransactions public immutable LINEA_ROLLUP;

  /// @notice Contains the destination chain ID used in the RLP encoding.
  uint256 public immutable DESTINATION_CHAIN_ID;

  /// @notice Contains the buffer for computing the L2 block the transaction will be processed by.
  uint256 public immutable L2_BLOCK_BUFFER;

  /// @notice Contains the l2 block time in seconds.
  uint256 public immutable L2_BLOCK_DURATION_SECONDS;
  /**
   * @notice Contains the minimum gas limit allowed for a forced transaction.
   * @dev Must be at least the worst-case intrinsic gas for the network's calldata and creation rules.
   *      Example (Osaka, MAX_INPUT_LENGTH_LIMIT = 1000 bytes, empty access list):
   *        21 000  base
   *      + 16 000  per-byte cost for 1 000 non-zero initcode bytes (16 gas each)
   *      + 32 000  contract-creation extra
   *      +     64  initcode word charge: 2 × ceil(1 000 / 32) = 2 × 32 words
   *      --------
   *        69 064  exact worst-case intrinsic gas
   *      The default deployment constant (70 000) is a conservative over-estimate of this figure.
   *      If the network's calldata byte limit or per-byte gas costs change, both this value
   *      and the prover's RLP-byte-size configuration must be updated together.
   */
  uint256 public immutable MIN_GAS_LIMIT;

  /// @notice Contains the maximum gas allowed for a forced transaction.
  uint256 public immutable MAX_GAS_LIMIT;

  /// @notice Contains the maximum calldata length allowed for a forced transaction.
  uint256 public immutable MAX_INPUT_LENGTH_LIMIT;

  /// @notice Contains the minimum base gas fee (maxFeePerGas) accepted for a forced transaction.
  /// @dev Set to zero for gasless networks, in which case zero EIP-1559 fee parameters are allowed.
  ///      When non-zero, the transaction's maxFeePerGas must be >= this value.
  uint256 public immutable MINIMUM_BASE_GAS_FEE;

  /// @notice Contains the buffer for the block number deadline if it is too low.
  /// @dev This is to accommodate the scenario where the next block deadline is lower than the previous one,
  ///      around the time of finalization.
  uint256 public immutable BLOCK_NUMBER_DEADLINE_BUFFER;

  /// @notice Contains the address for the transaction address filter.
  IAddressFilter public immutable ADDRESS_FILTER;

  /// @notice Toggles the feature switch for using the address filter.
  bool public useAddressFilter = true;

  /// @notice The L1 block number of the last submitted forced transaction.
  uint256 public lastSubmissionBlock;

  /**
   * @notice Initializes the forced transaction gateway.
   * @dev `_minimumBaseGasFee` can be set to zero for gasless networks. When zero, zero EIP-1559 fee
   *      parameters are allowed. When non-zero, the transaction's maxFeePerGas must be >= this value.
   * @param _rollup The Lineth rollup contract address.
   * @param _destinationChainId The L2 destination chain ID.
   * @param _l2BlockBuffer The L2 block buffer for forced transaction inclusion.
   * @param _minGasLimit The minimum gas limit for forced transactions. Must cover worst-case intrinsic
   *        gas for the configured calldata limit (see MIN_GAS_LIMIT NatSpec for the derivation).
   * @param _maxGasLimit The maximum gas limit allowed for forced transactions.
   * @param _maxInputLengthBuffer The maximum calldata length allowed for forced transactions.
   * @param _minimumBaseGasFee The minimum maxFeePerGas accepted; zero disables the check for gasless networks.
   * @param _defaultAdmin The account granted the default admin role.
   * @param _addressFilter The address filter contract address.
   * @param _l2BlockDurationSeconds The L2 block time in seconds.
   * @param _blockNumberDeadlineBuffer The buffer used when the computed block number deadline is too low.
   */
  constructor(
    address _rollup,
    uint256 _destinationChainId,
    uint256 _l2BlockBuffer,
    uint256 _minGasLimit,
    uint256 _maxGasLimit,
    uint256 _maxInputLengthBuffer,
    uint256 _minimumBaseGasFee,
    address _defaultAdmin,
    address _addressFilter,
    uint256 _l2BlockDurationSeconds,
    uint256 _blockNumberDeadlineBuffer
  ) {
    require(_rollup != address(0), IGenericErrors.ZeroAddressNotAllowed());
    require(_destinationChainId != 0, IGenericErrors.ZeroValueNotAllowed());
    require(_l2BlockBuffer != 0, IGenericErrors.ZeroValueNotAllowed());
    require(_minGasLimit != 0, IGenericErrors.ZeroValueNotAllowed());
    require(_maxGasLimit != 0, IGenericErrors.ZeroValueNotAllowed());
    require(_maxGasLimit > _minGasLimit, MaxGasLimitLessThanOrEqualToMinGasLimit(_minGasLimit, _maxGasLimit));
    require(_maxInputLengthBuffer != 0, IGenericErrors.ZeroValueNotAllowed());
    require(_defaultAdmin != address(0), IGenericErrors.ZeroAddressNotAllowed());
    require(_addressFilter != address(0), IGenericErrors.ZeroAddressNotAllowed());
    require(_l2BlockDurationSeconds != 0, IGenericErrors.ZeroValueNotAllowed());
    require(_blockNumberDeadlineBuffer != 0, IGenericErrors.ZeroValueNotAllowed());

    LINEA_ROLLUP = IAcceptForcedTransactions(_rollup);
    DESTINATION_CHAIN_ID = _destinationChainId;
    L2_BLOCK_BUFFER = _l2BlockBuffer;
    MIN_GAS_LIMIT = _minGasLimit;
    MAX_GAS_LIMIT = _maxGasLimit;
    MAX_INPUT_LENGTH_LIMIT = _maxInputLengthBuffer;
    MINIMUM_BASE_GAS_FEE = _minimumBaseGasFee;
    ADDRESS_FILTER = IAddressFilter(_addressFilter);
    L2_BLOCK_DURATION_SECONDS = _l2BlockDurationSeconds;
    BLOCK_NUMBER_DEADLINE_BUFFER = _blockNumberDeadlineBuffer;

    _grantRole(DEFAULT_ADMIN_ROLE, _defaultAdmin);
  }

  /**
   * @notice Function to submit forced transactions.
   * @param _forcedTransaction The fields required for the transaction excluding chainId.
   * @param _lastFinalizedState The last finalized state validated to use the timestamp in block number calculation.
   */
  function submitForcedTransaction(
    Eip1559Transaction memory _forcedTransaction,
    LastFinalizedState memory _lastFinalizedState
  ) external payable {
    require(_forcedTransaction.gasLimit >= MIN_GAS_LIMIT, GasLimitTooLow());
    require(_forcedTransaction.gasLimit <= MAX_GAS_LIMIT, MaxGasLimitExceeded());
    require(_forcedTransaction.input.length <= MAX_INPUT_LENGTH_LIMIT, CalldataInputLengthLimitExceeded());

    if (MINIMUM_BASE_GAS_FEE != 0) {
      require(
        _forcedTransaction.maxPriorityFeePerGas > 0 && _forcedTransaction.maxFeePerGas > 0,
        GasFeeParametersContainZero(_forcedTransaction.maxFeePerGas, _forcedTransaction.maxPriorityFeePerGas)
      );
      require(
        _forcedTransaction.maxFeePerGas >= MINIMUM_BASE_GAS_FEE,
        MaxFeePerGasLowerThanMinimumBaseGasFee(_forcedTransaction.maxFeePerGas, MINIMUM_BASE_GAS_FEE)
      );
    }

    require(
      _forcedTransaction.maxPriorityFeePerGas <= _forcedTransaction.maxFeePerGas,
      MaxPriorityFeePerGasHigherThanMaxFee(_forcedTransaction.maxFeePerGas, _forcedTransaction.maxPriorityFeePerGas)
    );

    require(_forcedTransaction.yParity <= 1, YParityGreaterThanOne(_forcedTransaction.yParity));

    require(block.number > lastSubmissionBlock, ForcedTransactionAlreadySubmittedInBlock(block.number));

    (
      bytes32 currentFinalizedState,
      bytes32 previousForcedTransactionRollingHash,
      uint256 previousForcedTransactionBlockDeadline,
      uint256 currentFinalizedL2BlockNumber,
      uint256 forcedTransactionFeeAmount
    ) = LINEA_ROLLUP.getRequiredForcedTransactionFields();

    require(msg.value == forcedTransactionFeeAmount, ForcedTransactionFeeNotMet(forcedTransactionFeeAmount, msg.value));

    if (
      currentFinalizedState !=
      FinalizedStateHashing._computeLastFinalizedState(
        _lastFinalizedState.messageNumber,
        _lastFinalizedState.messageRollingHash,
        _lastFinalizedState.forcedTransactionNumber,
        _lastFinalizedState.forcedTransactionRollingHash,
        _lastFinalizedState.timestamp
      )
    ) {
      revert FinalizationStateIncorrect(
        currentFinalizedState,
        FinalizedStateHashing._computeLastFinalizedState(
          _lastFinalizedState.messageNumber,
          _lastFinalizedState.messageRollingHash,
          _lastFinalizedState.forcedTransactionNumber,
          _lastFinalizedState.forcedTransactionRollingHash,
          _lastFinalizedState.timestamp
        )
      );
    }

    LibRLP.List memory transactionFieldList = LibRLP.p(DESTINATION_CHAIN_ID);
    transactionFieldList = LibRLP.p(transactionFieldList, _forcedTransaction.nonce);
    transactionFieldList = LibRLP.p(transactionFieldList, _forcedTransaction.maxPriorityFeePerGas);
    transactionFieldList = LibRLP.p(transactionFieldList, _forcedTransaction.maxFeePerGas);
    transactionFieldList = LibRLP.p(transactionFieldList, _forcedTransaction.gasLimit);

    if (_forcedTransaction.to == address(0)) {
      transactionFieldList = LibRLP.p(transactionFieldList, bytes(""));
    } else {
      transactionFieldList = LibRLP.p(transactionFieldList, _forcedTransaction.to);
    }
    transactionFieldList = LibRLP.p(transactionFieldList, _forcedTransaction.value);
    transactionFieldList = LibRLP.p(transactionFieldList, _forcedTransaction.input);
    transactionFieldList = LibRLP.p(transactionFieldList, LibRLP.p());

    bytes32 hashedPayload = keccak256(abi.encodePacked(hex"02", LibRLP.encode(transactionFieldList)));

    address signer;
    unchecked {
      signer = ECDSA.recover(
        hashedPayload,
        _forcedTransaction.yParity + 27,
        bytes32(_forcedTransaction.r),
        bytes32(_forcedTransaction.s)
      );
    }

    if (useAddressFilter) {
      require(!ADDRESS_FILTER.addressIsFiltered(signer), AddressIsFiltered());
      if (signer != _forcedTransaction.to) {
        require(!ADDRESS_FILTER.addressIsFiltered(_forcedTransaction.to), AddressIsFiltered());
      }
    }

    uint256 blockNumberDeadline;
    unchecked {
      /// @dev Converts elapsed time since last finalization to L2 blocks using the configured block time.
      blockNumberDeadline =
        currentFinalizedL2BlockNumber +
        (block.timestamp - _lastFinalizedState.timestamp) / L2_BLOCK_DURATION_SECONDS +
        L2_BLOCK_BUFFER;
    }

    if (blockNumberDeadline <= previousForcedTransactionBlockDeadline) {
      blockNumberDeadline = previousForcedTransactionBlockDeadline + BLOCK_NUMBER_DEADLINE_BUFFER;
    }

    bytes32 hashedPayloadMsb;
    bytes32 hashedPayloadLsb;
    assembly {
      hashedPayloadMsb := shr(128, hashedPayload)
      hashedPayloadLsb := and(hashedPayload, 0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF)
    }

    transactionFieldList = LibRLP.p(transactionFieldList, _forcedTransaction.yParity);
    transactionFieldList = LibRLP.p(transactionFieldList, _forcedTransaction.r);
    transactionFieldList = LibRLP.p(transactionFieldList, _forcedTransaction.s);

    LINEA_ROLLUP.storeForcedTransaction{ value: msg.value }(
      Mimc.hash(
        abi.encode(
          previousForcedTransactionRollingHash,
          hashedPayloadMsb,
          hashedPayloadLsb,
          blockNumberDeadline,
          signer
        )
      ),
      signer,
      blockNumberDeadline,
      abi.encodePacked(hex"02", LibRLP.encode(transactionFieldList))
    );

    lastSubmissionBlock = block.number;
  }

  /**
   * @notice Function to toggle the usage of the address filter.
   * @dev Only callable by an account with the DEFAULT_ADMIN_ROLE.
   * @param _useAddressFilter Bool indicating whether or not to use the address filter.
   */
  function toggleUseAddressFilter(bool _useAddressFilter) external onlyRole(DEFAULT_ADMIN_ROLE) {
    require(useAddressFilter != _useAddressFilter, AddressFilterAlreadySet(_useAddressFilter));
    useAddressFilter = _useAddressFilter;
    emit AddressFilterSet(_useAddressFilter);
  }
}
