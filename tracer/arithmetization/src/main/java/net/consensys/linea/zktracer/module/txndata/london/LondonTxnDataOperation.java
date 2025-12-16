/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.txndata.london;

import static net.consensys.linea.zktracer.Trace.EIP2681_MAX_NONCE;
import static net.consensys.linea.zktracer.Trace.MAX_REFUND_QUOTIENT;
import static net.consensys.linea.zktracer.Trace.RLP_RCPT_SUBPHASE_ID_CUMUL_GAS;
import static net.consensys.linea.zktracer.Trace.RLP_RCPT_SUBPHASE_ID_STATUS_CODE;
import static net.consensys.linea.zktracer.Trace.RLP_RCPT_SUBPHASE_ID_TYPE;
import static net.consensys.linea.zktracer.TraceLondon.Txndata.*;
import static net.consensys.linea.zktracer.module.Util.getTxTypeAsInt;
import static net.consensys.linea.zktracer.types.AddressUtils.highPart;
import static net.consensys.linea.zktracer.types.AddressUtils.lowPart;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.booleanToInt;
import static org.hyperledger.besu.datatypes.TransactionType.*;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import lombok.Getter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.txndata.BlockSnapshot;
import net.consensys.linea.zktracer.module.txndata.TxnDataOperation;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.TransactionType;

public class LondonTxnDataOperation extends TxnDataOperation {
  protected final Wcp wcp;
  protected final Euc euc;
  @Getter public final TransactionProcessingMetadata tx;

  private static final Bytes EIP_2681_MAX_NONCE = bigIntegerToBytes(EIP2681_MAX_NONCE);

  private final int nbRowsType0;
  private final int nbRowsType1;
  private final int nbRowsType2;
  private final int nbRowsTxMax;
  private final int nbWcpEucRowsFrontierAccessList;
  protected final List<TxnDataComparisonRecord> callsToEucAndWcp;
  private final ArrayList<RlptxnOutgoing> valuesToRlptxn;
  private final ArrayList<RlptxrcptOutgoing> valuesToRlpTxrcpt;
  private static final Bytes BYTES_MAX_REFUND_QUOTIENT = Bytes.of(MAX_REFUND_QUOTIENT);

  public LondonTxnDataOperation(
      Wcp wcp,
      Euc euc,
      TransactionProcessingMetadata tx,
      int nbRowsType0,
      int nbRowsType1,
      int nbRowsType2,
      int nbWcpEucRowsFrontierAccessList) {

    this.wcp = wcp;
    this.euc = euc;
    this.tx = tx;

    // The number of rows type0, type1, type2 and thereforeTxMax depends on the fork so the
    // parameters are passed in constructor and set
    // to be used in setter functions
    this.nbRowsType0 = nbRowsType0;
    this.nbRowsType1 = nbRowsType1;
    this.nbRowsType2 = nbRowsType2;
    this.nbRowsTxMax = Math.max(Math.max(nbRowsType0, nbRowsType1), nbRowsType2);
    // The number of wcp and euc for frontier and access list type depends on the fork
    // (post shanghai) two rows are added for limit and meter initcode
    this.nbWcpEucRowsFrontierAccessList = nbWcpEucRowsFrontierAccessList;
    this.callsToEucAndWcp = new ArrayList<>(nbRowsTxMax);
    this.valuesToRlptxn = new ArrayList<>(nbRowsTxMax);
    this.valuesToRlpTxrcpt = new ArrayList<>(nbRowsTxMax);

    this.setCallsToEucAndWcp();
  }

  private void setCallsToEucAndWcp() {

    // row 0: nonce VS. EIP-2681 max nonce
    final Bytes nonce = Bytes.minimalBytes(tx.getBesuTransaction().getNonce());
    callsToEucAndWcp.add(TxnDataComparisonRecord.callToLt(wcp, nonce, EIP_2681_MAX_NONCE));

    // row 1: initial balance covers the upfront wei cost
    final Bytes initialBalance = bigIntegerToBytes(tx.getInitialBalance());
    final BigInteger value = tx.getBesuTransaction().getValue().getAsBigInteger();
    final Bytes upfrontWeiCost =
        bigIntegerToBytes(
            value.add(
                outgoingLowRow6()
                    .multiply(BigInteger.valueOf(tx.getBesuTransaction().getGasLimit()))));
    callsToEucAndWcp.add(TxnDataComparisonRecord.callToLeq(wcp, upfrontWeiCost, initialBalance));

    // Row 2 & 3 (post Shanghai): limit and meter initcode
    setShanghaiCallsToEucAndWcp();

    // row 2: gasLimit covers the upfront gas cost
    final Bytes gasLimit = Bytes.minimalBytes(tx.getBesuTransaction().getGasLimit());
    final Bytes upfrontGasCost = Bytes.minimalBytes(tx.getUpfrontGasCost());
    callsToEucAndWcp.add(TxnDataComparisonRecord.callToLeq(wcp, upfrontGasCost, gasLimit));

    // row 3: computing upper limit for refunds
    final Bytes gasConsumedByTransactionExecution =
        Bytes.minimalBytes(tx.getBesuTransaction().getGasLimit() - tx.getLeftoverGas());
    final TxnDataComparisonRecord refundLimitComputation =
        TxnDataComparisonRecord.callToEuc(
            euc, gasConsumedByTransactionExecution, BYTES_MAX_REFUND_QUOTIENT);
    final Bytes refundLimit = refundLimitComputation.result();
    callsToEucAndWcp.add(refundLimitComputation);

    // row 4: comparing accrued refunds to the upper limit of refunds
    final Bytes accruedRefunds = Bytes.minimalBytes(tx.getRefundCounterMax());
    callsToEucAndWcp.add(TxnDataComparisonRecord.callToLt(wcp, accruedRefunds, refundLimit));

    // row 5: detecting empty payload
    final Bytes payloadSize = Bytes.minimalBytes(tx.getBesuTransaction().getPayload().size());
    callsToEucAndWcp.add(TxnDataComparisonRecord.callToIszero(wcp, payloadSize));

    // row 6: comparing the maximal gas price against the base fee
    final TransactionType type = tx.getBesuTransaction().getType();
    final Bytes baseFee = Bytes.minimalBytes(tx.getBaseFee());
    if (type == FRONTIER || type == ACCESS_LIST) {
      final Bytes gasPriceBytes =
          bigIntegerToBytes(tx.getBesuTransaction().getGasPrice().get().getAsBigInteger());
      callsToEucAndWcp.add(TxnDataComparisonRecord.callToLeq(wcp, baseFee, gasPriceBytes));
    }

    switch (type) {
      case FRONTIER -> {
        for (int i = this.nbWcpEucRowsFrontierAccessList; i < this.nbRowsType0; i++) {
          callsToEucAndWcp.add(TxnDataComparisonRecord.empty());
        }
      }
      case ACCESS_LIST -> {
        for (int i = this.nbWcpEucRowsFrontierAccessList; i < this.nbRowsType1; i++) {
          callsToEucAndWcp.add(TxnDataComparisonRecord.empty());
        }
      }
      case EIP1559 -> {

        // row 6: comparing the maximal gas price against the base fee
        final Bytes maxFeePerGas =
            bigIntegerToBytes(tx.getBesuTransaction().getMaxFeePerGas().get().getAsBigInteger());
        callsToEucAndWcp.add(TxnDataComparisonRecord.callToLeq(wcp, baseFee, maxFeePerGas));

        // row 7: comparing max fee to the max priority fee
        final BigInteger maxPriorityFeePerGas =
            tx.getBesuTransaction().getMaxPriorityFeePerGas().get().getAsBigInteger();
        callsToEucAndWcp.add(
            TxnDataComparisonRecord.callToLeq(
                wcp, bigIntegerToBytes(maxPriorityFeePerGas), maxFeePerGas));

        // row 8: computing the effective gas price
        final Bytes maxPriorityFeePerGasPlusBaseFee =
            bigIntegerToBytes(maxPriorityFeePerGas.add(BigInteger.valueOf(tx.getBaseFee())));
        callsToEucAndWcp.add(
            TxnDataComparisonRecord.callToLeq(wcp, maxPriorityFeePerGasPlusBaseFee, maxFeePerGas));
      }
    }
  }

  public void setShanghaiCallsToEucAndWcp() {}

  private void setRlptxnValues() {
    // i+0
    valuesToRlptxn.add(
        RlptxnOutgoing.set(
            (short) COMMON_RLP_TXN_PHASE_NUMBER_0,
            Bytes.EMPTY,
            Bytes.ofUnsignedInt(getTxTypeAsInt(tx.getBesuTransaction().getType()))));
    // i+1
    valuesToRlptxn.add(
        RlptxnOutgoing.set(
            (short) COMMON_RLP_TXN_PHASE_NUMBER_1,
            tx.isDeployment() ? Bytes.EMPTY : tx.getEffectiveRecipient().slice(0, 4),
            tx.isDeployment() ? Bytes.EMPTY : lowPart(tx.getEffectiveRecipient())));

    // i+2
    valuesToRlptxn.add(
        RlptxnOutgoing.set(
            (short) COMMON_RLP_TXN_PHASE_NUMBER_2,
            Bytes.EMPTY,
            Bytes.ofUnsignedLong(tx.getBesuTransaction().getNonce())));

    // i+3
    valuesToRlptxn.add(
        RlptxnOutgoing.set(
            (short) COMMON_RLP_TXN_PHASE_NUMBER_3,
            tx.isDeployment() ? Bytes.of(1) : Bytes.EMPTY,
            bigIntegerToBytes(tx.getBesuTransaction().getValue().getAsBigInteger())));

    // i+4
    valuesToRlptxn.add(
        RlptxnOutgoing.set(
            (short) COMMON_RLP_TXN_PHASE_NUMBER_4,
            Bytes.ofUnsignedLong(tx.getDataCost()),
            Bytes.ofUnsignedLong(tx.getBesuTransaction().getPayload().size())));

    // i+5
    valuesToRlptxn.add(
        RlptxnOutgoing.set(
            (short) COMMON_RLP_TXN_PHASE_NUMBER_5,
            Bytes.EMPTY,
            Bytes.ofUnsignedLong(tx.getBesuTransaction().getGasLimit())));

    switch (tx.getBesuTransaction().getType()) {
      case FRONTIER -> {
        // i+6
        valuesToRlptxn.add(
            RlptxnOutgoing.set(
                (short) TYPE_0_RLP_TXN_PHASE_NUMBER_6,
                Bytes.EMPTY,
                Bytes.minimalBytes(tx.getEffectiveGasPrice())));
        for (int i = 7; i < this.nbRowsType0 + 1; i++) {
          valuesToRlptxn.add(RlptxnOutgoing.empty());
        }
      }
      case ACCESS_LIST -> {
        // i+6
        valuesToRlptxn.add(
            RlptxnOutgoing.set(
                (short) TYPE_1_RLP_TXN_PHASE_NUMBER_6,
                Bytes.EMPTY,
                Bytes.minimalBytes(tx.getEffectiveGasPrice())));

        // i+7
        valuesToRlptxn.add(
            RlptxnOutgoing.set(
                (short) TYPE_1_RLP_TXN_PHASE_NUMBER_7,
                Bytes.ofUnsignedInt(tx.numberOfWarmedStorageKeys()),
                Bytes.ofUnsignedInt(tx.numberOfWarmedAddresses())));

        for (int i = 8; i < this.nbRowsType1 + 1; i++) {
          valuesToRlptxn.add(RlptxnOutgoing.empty());
        }
      }

      case EIP1559 -> {
        // i+6
        valuesToRlptxn.add(
            RlptxnOutgoing.set(
                (short) TYPE_2_RLP_TXN_PHASE_NUMBER_6,
                bigIntegerToBytes(
                    tx.getBesuTransaction().getMaxPriorityFeePerGas().get().getAsBigInteger()),
                bigIntegerToBytes(outgoingLowRow6())));

        // i+7
        valuesToRlptxn.add(
            RlptxnOutgoing.set(
                (short) TYPE_2_RLP_TXN_PHASE_NUMBER_7,
                Bytes.ofUnsignedInt(tx.numberOfWarmedStorageKeys()),
                Bytes.ofUnsignedInt(tx.numberOfWarmedAddresses())));

        for (int i = 8; i < this.nbRowsType2 + 1; i++) {
          valuesToRlptxn.add(RlptxnOutgoing.empty());
        }
      }
    }
  }

  public void setRlptxrcptValues() {
    // i+0
    this.valuesToRlpTxrcpt.add(
        RlptxrcptOutgoing.set(
            (short) RLP_RCPT_SUBPHASE_ID_TYPE, getTxTypeAsInt(tx.getBesuTransaction().getType())));
    // i+1
    this.valuesToRlpTxrcpt.add(
        RlptxrcptOutgoing.set(
            (short) RLP_RCPT_SUBPHASE_ID_STATUS_CODE, booleanToInt(tx.statusCode())));
    // i+2
    this.valuesToRlpTxrcpt.add(
        RlptxrcptOutgoing.set(
            (short) RLP_RCPT_SUBPHASE_ID_CUMUL_GAS, tx.getAccumulatedGasUsedInBlock()));
    // i+3 to i+nbRowsTxMax
    for (int ct = 3; ct < this.nbRowsTxMax; ct++) {
      this.valuesToRlpTxrcpt.add(RlptxrcptOutgoing.emptyValue());
    }
  }

  public void setCallWcpLastTxOfBlock(final Bytes blockGasLimit) {
    final Bytes arg1 = Bytes.minimalBytes(tx.getAccumulatedGasUsedInBlock());
    callsToEucAndWcp.add(TxnDataComparisonRecord.callToLeq(wcp, arg1, blockGasLimit));
  }

  private BigInteger outgoingLowRow6() {
    return switch (tx.getBesuTransaction().getType()) {
      case FRONTIER, ACCESS_LIST -> tx.getBesuTransaction().getGasPrice().get().getAsBigInteger();
      case EIP1559 -> tx.getBesuTransaction().getMaxFeePerGas().get().getAsBigInteger();
      default ->
          throw new RuntimeException(
              "Transaction type not supported:" + tx.getBesuTransaction().getType());
    };
  }

  public void traceTransaction(Trace.Txndata trace, BlockSnapshot block, int absTxNumMax) {

    this.setRlptxnValues();
    this.setRlptxrcptValues();

    final boolean isLastTxOfTheBlock =
        tx.getRelativeTransactionNumber() == block.getNbOfTxsInBlock();
    if (isLastTxOfTheBlock) {
      valuesToRlptxn.add(RlptxnOutgoing.empty());
      valuesToRlpTxrcpt.add(RlptxrcptOutgoing.emptyValue());
    }

    final long fromHi = highPart(tx.getSender());
    final Bytes fromLo = lowPart(tx.getSender());
    final Bytes nonce = Bytes.ofUnsignedLong(tx.getBesuTransaction().getNonce());
    final Bytes initialBalance = bigIntegerToBytes(tx.getInitialBalance());
    final Bytes value = bigIntegerToBytes(tx.getBesuTransaction().getValue().getAsBigInteger());
    final long toHi = highPart(tx.getEffectiveRecipient());
    final Bytes toLo = lowPart(tx.getEffectiveRecipient());
    final Bytes gasLimit = Bytes.minimalBytes(tx.getBesuTransaction().getGasLimit());
    final Bytes gasInitiallyAvailable = Bytes.minimalBytes(tx.getInitiallyAvailableGas());
    final Bytes gasPrice = Bytes.minimalBytes(tx.getEffectiveGasPrice());
    final Bytes priorityFeePerGas = Bytes.minimalBytes(tx.feeRateForCoinbase());
    final Bytes baseFee = block.getBaseFee().get().toMinimalBytes();
    final long coinbaseHi = highPart(tx.getCoinbaseAddress());
    final Bytes coinbaseLo = lowPart(tx.getCoinbaseAddress());
    final int callDataSize = tx.isDeployment() ? 0 : tx.getBesuTransaction().getPayload().size();
    final int initCodeSize = tx.isDeployment() ? tx.getBesuTransaction().getPayload().size() : 0;
    final Bytes gasLeftOver = Bytes.minimalBytes(tx.getLeftoverGas());
    final Bytes refundCounter = Bytes.minimalBytes(tx.getRefundCounterMax());
    final Bytes refundEffective = Bytes.minimalBytes(tx.getGasRefunded());
    final Bytes cumulativeGas = Bytes.minimalBytes(tx.getAccumulatedGasUsedInBlock());

    final int nbLInes = isLastTxOfTheBlock ? this.lineCount() + 1 : this.lineCount();

    for (int ct = 0; ct < nbLInes; ct++) {
      trace
          .absTxNumMax(absTxNumMax)
          .absTxNum(tx.getUserTransactionNumber())
          .relBlock(tx.getRelativeBlockNumber())
          .relTxNumMax(block.getNbOfTxsInBlock())
          .relTxNum(tx.getRelativeTransactionNumber())
          .isLastTxOfBlock(isLastTxOfTheBlock)
          .ct(UnsignedByte.of(ct))
          .fromHi(fromHi)
          .fromLo(fromLo)
          .nonce(nonce)
          .initialBalance(initialBalance)
          .value(value)
          .toHi(toHi)
          .toLo(toLo)
          .isDep(tx.isDeployment())
          .gasLimit(gasLimit.toLong())
          .gasInitiallyAvailable(gasInitiallyAvailable.toLong())
          .gasPrice(gasPrice)
          .priorityFeePerGas(priorityFeePerGas)
          .basefee(baseFee)
          .coinbaseHi(coinbaseHi)
          .coinbaseLo(coinbaseLo)
          .blockGasLimit(block.getBlockGasLimit())
          .callDataSize(callDataSize)
          .initCodeSize(initCodeSize)
          .type0(tx.getBesuTransaction().getType() == FRONTIER)
          .type1(tx.getBesuTransaction().getType() == ACCESS_LIST)
          .type2(tx.getBesuTransaction().getType() == TransactionType.EIP1559)
          .requiresEvmExecution(tx.requiresEvmExecution())
          .copyTxcd(tx.copyTransactionCallData())
          .gasLeftover(gasLeftOver.toLong())
          .refundCounter(refundCounter.toLong())
          .refundEffective(refundEffective.toLong())
          .gasCumulative(cumulativeGas)
          .statusCode(tx.statusCode())
          .codeFragmentIndex(tx.getCodeFragmentIndex())
          .phaseRlpTxn(UnsignedByte.of(valuesToRlptxn.get(ct).phase()))
          .outgoingHi(valuesToRlptxn.get(ct).outGoingHi())
          .outgoingLo(valuesToRlptxn.get(ct).outGoingLo())
          .eucFlag(callsToEucAndWcp.get(ct).eucFlag())
          .wcpFlag(callsToEucAndWcp.get(ct).wcpFlag())
          .inst(UnsignedByte.of(callsToEucAndWcp.get(ct).instruction()))
          .argOneLo(callsToEucAndWcp.get(ct).arg1())
          .argTwoLo(callsToEucAndWcp.get(ct).arg2())
          .res(callsToEucAndWcp.get(ct).result())
          .phaseRlpTxnrcpt(UnsignedByte.of(valuesToRlpTxrcpt.get(ct).phase()))
          .outgoingRlpTxnrcpt(Bytes.ofUnsignedLong(valuesToRlpTxrcpt.get(ct).outgoing()))
          .validateRow();
    }
  }

  @Override
  public void traceTransaction(Trace.Txndata trace) {}

  @Override
  protected int computeLineCount() {
    // Count the number of rows of each tx, only depending on the type of the transaction
    return switch (tx.getBesuTransaction().getType()) {
      case FRONTIER -> NB_ROWS_TYPE_0;
      case ACCESS_LIST -> NB_ROWS_TYPE_1;
      case EIP1559 -> NB_ROWS_TYPE_2;
      default ->
          throw new RuntimeException(
              "Transaction type not supported:" + tx.getBesuTransaction().getType());
    };
  }
}
