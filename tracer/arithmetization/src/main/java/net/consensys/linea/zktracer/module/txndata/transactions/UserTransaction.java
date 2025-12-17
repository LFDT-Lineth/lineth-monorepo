/*
 * Copyright Consensys Software Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
 * the License. You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * SPDX-License-Identifier: Apache-2.0
 */
package net.consensys.linea.zktracer.module.txndata.transactions;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.TraceOsaka.EIP_7825_TRANSACTION_GAS_LIMIT_CAP;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.USER;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import java.math.BigInteger;
import lombok.Getter;
import net.consensys.linea.zktracer.module.txndata.TxnData;
import net.consensys.linea.zktracer.module.txndata.TxnDataOperation;
import net.consensys.linea.zktracer.module.txndata.rows.RlpRow;
import net.consensys.linea.zktracer.module.txndata.rows.computationRows.EucRow;
import net.consensys.linea.zktracer.module.txndata.rows.computationRows.WcpRow;
import net.consensys.linea.zktracer.module.txndata.rows.hubRows.HubRowForUserTransactions;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public class UserTransaction extends TxnDataOperation {
  public static final short NB_ROWS_TXN_DATA_OSAKA_USER_1559_SEMANTIC = 17;
  public static final short NB_ROWS_TXN_DATA_OSAKA_USER_NO_1559_SEMANTIC = 15;
  private static final Bytes EIP_2681_MAX_NONCE = bigIntegerToBytes(EIP2681_MAX_NONCE);
  private static final Bytes EIP_7825_TRANSACTION_GAS_LIMIT_CAP_BYTES =
      Bytes.minimalBytes(EIP_7825_TRANSACTION_GAS_LIMIT_CAP);
  public static final Bytes MAX_INIT_CODE_SIZE_BYTES = Bytes.ofUnsignedInt(MAX_INIT_CODE_SIZE);
  public final TransactionProcessingMetadata txn;
  public final ProcessableBlockHeader blockHeader;

  public enum DominantCost {
    FLOOR_COST_DOMINATES,
    EXECUTION_COST_DOMINATES
  }

  @Getter private DominantCost dominantCost;

  public UserTransaction(final TxnData txnData, final TransactionProcessingMetadata txnMetadata) {
    super(txnData, USER);

    this.blockHeader = txnData.currentBlockHeader();
    this.txn = txnMetadata;

    this.process();
  }

  /**
   * Every line of function call in the {@link UserTransaction#process} method corresponds to a row
   * in the USER transaction processing of the specification.
   */
  private void process() {

    hubRow();
    rlpRow();
    maxNonceCheckComputationRow();
    initialBalanceCheckComputationRow();
    maxInitCodeSizeCheckComputationRow();
    initCodePricingComputationRow();
    gasLimitMustCoverTheUpfrontGasCostComputationRow();
    eip7825TransactionGasLimitCap();
    gasLimitMustCoverTheTransactionFloorCostComputationRow();
    final long upperLimitForGasRefunds = upperLimitForGasRefundsComputationRow();
    final long consumedGasAfterRefunds = effectiveRefundsComputationRow(upperLimitForGasRefunds);
    comparingEffectiveRefundToFloorCostComputationRow(consumedGasAfterRefunds);
    detectingEmptyPayloadComputationRow();
    comparingTheMaximumGasPriceToTheBaseFee();
    cumulativeGasConsumptionMustNotExceedBlockGasLimitComputationRow();

    if (txn.transactionTypeHasEip1559GasSemantics()) {
      comparingMaxFeeToMaxPriorityFeeComputationRow();
      computingTheEffectiveGasPriceComputationRow();
    }
  }

  @Override
  protected int ctMax() {
    return (txn.transactionTypeHasEip1559GasSemantics()
            ? NB_ROWS_TXN_DATA_OSAKA_USER_1559_SEMANTIC
            : NB_ROWS_TXN_DATA_OSAKA_USER_NO_1559_SEMANTIC)
        - 1;
  }

  void hubRow() {
    rows.add(new HubRowForUserTransactions(blockHeader, txn));
  }

  void rlpRow() {
    rows.add(new RlpRow(txn));
  }

  /** Performs the EIP-2681 check that the nonce is less than 2^64 - 1. */
  private void maxNonceCheckComputationRow() {
    WcpRow maxNonceCheckEip2681 =
        WcpRow.smallCallToLt(
            wcp, Bytes.ofUnsignedLong(txn.getBesuTransaction().getNonce()), EIP_2681_MAX_NONCE);
    checkArgument(maxNonceCheckEip2681.result(), "Transaction nonce is too high");
    rows.add(maxNonceCheckEip2681);
  }

  /**
   * Ensures that the initial balance of the sender account is sufficient to cover the maximum
   * possible cost of the transaction (value + gas_limit * max(gas_price, max_fee)).
   */
  private void initialBalanceCheckComputationRow() {
    final Bytes initialBalance = bigIntegerToBytes(txn.getInitialBalance());
    final BigInteger value = txn.getBesuTransaction().getValue().getAsBigInteger();
    final BigInteger maxGasPrice =
        txn.transactionTypeHasEip1559GasSemantics()
            ? txn.getBesuTransaction().getMaxFeePerGas().get().getAsBigInteger()
            : txn.getBesuTransaction().getGasPrice().get().getAsBigInteger();
    final Bytes maxCostInWei =
        bigIntegerToBytes(value.add(maxGasPrice.multiply(BigInteger.valueOf(txn.getGasLimit()))));
    final WcpRow initialBalanceMustCoverValueAndGas =
        WcpRow.smallCallToLeq(wcp, maxCostInWei, initialBalance);
    checkArgument(
        initialBalanceMustCoverValueAndGas.result(),
        "Initial balance %s does not cover the max value and gas cost %s",
        initialBalance,
        maxCostInWei);

    rows.add(initialBalanceMustCoverValueAndGas);
  }

  /** Performs the EIP-3860 check that the init code size is at most 49152 bytes. */
  private void maxInitCodeSizeCheckComputationRow() {
    final int initCodeSize = initCodeSize();
    final WcpRow eip3860requiredInitCodeSizeCheck =
        WcpRow.smallCallToLeq(wcp, Bytes.ofUnsignedInt(initCodeSize), MAX_INIT_CODE_SIZE_BYTES);
    checkArgument(
        eip3860requiredInitCodeSizeCheck.result(),
        "Init code size %s exceeds the EIP-3860 limit of %s bytes",
        initCodeSize,
        MAX_INIT_CODE_SIZE_BYTES);
    rows.add(eip3860requiredInitCodeSizeCheck);
  }

  /** Adds the EIP-3860-induced init code pricing computation row. */
  private void initCodePricingComputationRow() {
    // Deployment transaction
    final long dividend = initCodeSize() + ((long) WORD_SIZE_MO);
    rows.add(EucRow.callToEuc(euc, dividend, WORD_SIZE));
  }

  private void gasLimitMustCoverTheUpfrontGasCostComputationRow() {
    final long upfrontGasCost = txn.getUpfrontGasCost();
    final long gasLimit = txn.getGasLimit();

    final WcpRow gasLimitMustCoverUpfrontGasCost =
        WcpRow.smallCallToLeq(
            wcp, Bytes.ofUnsignedLong(upfrontGasCost), Bytes.ofUnsignedLong(gasLimit));

    checkArgument(
        gasLimitMustCoverUpfrontGasCost.result(),
        "Gas limit %s does not cover the upfront gas cost %s",
        gasLimit,
        upfrontGasCost);

    rows.add(gasLimitMustCoverUpfrontGasCost);
  }

  /** EIP-7825 is in Osaka and add a transaction gas limit cap */
  protected void eip7825TransactionGasLimitCap() {
    final long gasLimit = txn.getGasLimit();

    final WcpRow transactionGasLimitCap =
        WcpRow.smallCallToLeq(
            wcp, Bytes.ofUnsignedLong(gasLimit), EIP_7825_TRANSACTION_GAS_LIMIT_CAP_BYTES);

    rows.add(transactionGasLimitCap);
  }

  private void gasLimitMustCoverTheTransactionFloorCostComputationRow() {
    final long floorGasCost =
        txn.getFloorCostPrague(); // Always use the Prague floor cost, even for Cancun
    final long gasLimit = txn.getGasLimit();

    final WcpRow gasLimitMustCoverFloorGasCost =
        WcpRow.smallCallToLeq(
            wcp, Bytes.ofUnsignedLong(floorGasCost), Bytes.ofUnsignedLong(gasLimit));
    rows.add(gasLimitMustCoverFloorGasCost);
  }

  private long upperLimitForGasRefundsComputationRow() {
    final long executionGasCost = txn.getGasLimit() - txn.getLeftoverGas();
    final EucRow upperLimitForGasRefunds =
        EucRow.callToEuc(euc, executionGasCost, MAX_REFUND_QUOTIENT);

    rows.add(upperLimitForGasRefunds);

    return upperLimitForGasRefunds.quotient();
  }

  private long effectiveRefundsComputationRow(long upperLimitForGasRefunds) {
    final WcpRow effectiveRefunds =
        WcpRow.smallCallToLt(
            wcp,
            Bytes.ofUnsignedLong(txn.getRefundCounterMax()),
            Bytes.ofUnsignedLong(upperLimitForGasRefunds));

    final boolean accruedRefundsAreLessThanTheRefundUpperLimit = effectiveRefunds.result();
    final long consumedGasAfterRefunds =
        accruedRefundsAreLessThanTheRefundUpperLimit
            ? txn.getGasLimit() - txn.getLeftoverGas() - txn.getRefundCounterMax()
            : txn.getGasLimit() - txn.getLeftoverGas() - upperLimitForGasRefunds;

    rows.add(effectiveRefunds);

    return consumedGasAfterRefunds;
  }

  private void comparingEffectiveRefundToFloorCostComputationRow(long consumedGasAfterRefunds) {
    final WcpRow comparingEffectiveRefundsVsFloorCost =
        WcpRow.smallCallToLt(
            wcp,
            Bytes.ofUnsignedLong(consumedGasAfterRefunds),
            Bytes.ofUnsignedLong(txn.getFloorCostPrague()));

    rows.add(comparingEffectiveRefundsVsFloorCost);
    dominantCost =
        comparingEffectiveRefundsVsFloorCost.result()
            ? DominantCost.FLOOR_COST_DOMINATES
            : DominantCost.EXECUTION_COST_DOMINATES;
  }

  private void detectingEmptyPayloadComputationRow() {
    final WcpRow detectingEmptyCallData =
        WcpRow.smallCallToIszero(wcp, txn.getBesuTransaction().getPayload().size());

    rows.add(detectingEmptyCallData);
  }

  private void comparingTheMaximumGasPriceToTheBaseFee() {
    final long gasPriceOrZero = txn.gasPrice().toLong();
    final long maxFeePerGasOrZero = txn.maxFeePerGas().toLong();
    final long maximumGasPrice =
        txn.transactionTypeHasEip1559GasSemantics() ? maxFeePerGasOrZero : gasPriceOrZero;
    final long baseFee = txn.getBaseFee();

    final WcpRow maximumGasPriceVsBaseFee = WcpRow.smallCallToLeq(wcp, baseFee, maximumGasPrice);

    final String errorMessage =
        txn.transactionTypeHasEip1559GasSemantics()
            ? String.format(
                "Maximum fee per gas %s is less than the block base fee %s",
                maximumGasPrice, baseFee)
            : String.format(
                "Gas price %s is less than the block base fee %s", maximumGasPrice, baseFee);
    checkArgument(maximumGasPriceVsBaseFee.result(), errorMessage);

    rows.add(maximumGasPriceVsBaseFee);
  }

  private void cumulativeGasConsumptionMustNotExceedBlockGasLimitComputationRow() {
    final WcpRow cumulativeGasConsumptionMustNotExceedBlockGasLimit =
        WcpRow.smallCallToLeq(wcp, txn.getAccumulatedGasUsedInBlock(), blockHeader.getGasLimit());

    checkArgument(
        cumulativeGasConsumptionMustNotExceedBlockGasLimit.result(),
        "Cumulative gas consumption %s exceeds the block gas limit %s",
        txn.getAccumulatedGasUsedInBlock(),
        txn.getGasLimit());

    rows.add(cumulativeGasConsumptionMustNotExceedBlockGasLimit);
  }

  private void comparingMaxFeeToMaxPriorityFeeComputationRow() {
    final long maxFeePerGas = txn.maxFeePerGas().toLong();
    final long maxPriorityFeePerGas = txn.maxPriorityFeePerGas().toLong();

    final WcpRow comparingMaxFeeToMaxPriorityFee =
        WcpRow.smallCallToLeq(
            wcp, Bytes.ofUnsignedLong(maxPriorityFeePerGas), Bytes.ofUnsignedLong(maxFeePerGas));

    checkArgument(
        comparingMaxFeeToMaxPriorityFee.result(),
        "Max priority fee per gas %s exceeds max fee per gas %s",
        maxPriorityFeePerGas,
        maxFeePerGas);

    rows.add(comparingMaxFeeToMaxPriorityFee);
  }

  private void computingTheEffectiveGasPriceComputationRow() {
    final long maxPriorityFeePerGas = txn.maxPriorityFeePerGas().toLong();
    final long maxFeePerGas = txn.maxFeePerGas().toLong();

    final WcpRow computingTheEffectiveGasPrice =
        WcpRow.smallCallToLeq(wcp, maxPriorityFeePerGas + txn.getBaseFee(), maxFeePerGas);

    rows.add(computingTheEffectiveGasPrice);
  }

  private int initCodeSize() {
    return txn.isDeployment() ? txn.getBesuTransaction().getPayload().size() : 0;
  }
}
