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

package net.consensys.linea.zktracer.module.rlptxn.phaseSection;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;

@RequiredArgsConstructor
public abstract class PhaseSection {

  public void trace(Trace.Rlptxn trace, GenericTracedValue tracedValues) {
    traceTransactionRow(trace, tracedValues.tx(), tracedValues);
    traceComputationRows(trace, tracedValues.tx(), tracedValues);
  }

  private void traceTransactionRow(
      Trace.Rlptxn trace, TransactionProcessingMetadata tx, GenericTracedValue tracedValues) {
    traceTransactionConstantValues(trace, tracedValues);
    traceLtLx(trace);
    trace
        .txn(true)
        .pTxnTxType(tx.type())
        .pTxnChainId(tx.chainId())
        .pTxnNonce(Bytes.ofUnsignedLong(tx.getBesuTransaction().getNonce()))
        .pTxnGasPrice(tx.gasPrice())
        .pTxnMaxPriorityFeePerGas(tx.maxPriorityFeePerGas())
        .pTxnMaxFeePerGas(tx.maxFeePerGas())
        .pTxnGasLimit(tx.getBesuTransaction().getGasLimit())
        .pTxnToHi(
            tx.isDeployment() ? 0 : tx.getBesuTransaction().getTo().get().slice(0, 4).toLong())
        .pTxnToLo(
            tx.isDeployment()
                ? Bytes.EMPTY
                : tx.getBesuTransaction().getTo().get().slice(4, LLARGE))
        .pTxnValue(bigIntegerToBytes(tx.getBesuTransaction().getValue().getAsBigInteger()))
        .pTxnNumberOfZeroBytes(tx.numberOfZeroBytesInPayload())
        .pTxnNumberOfNonzeroBytes(tx.numberOfNonzeroBytesInPayload())
        .pTxnNumberOfPrewarmedAddresses(tx.numberOfWarmedAddresses())
        .pTxnNumberOfPrewarmedStorageKeys(tx.numberOfWarmedStorageKeys());
    tracePostValues(trace, tracedValues);
  }

  protected abstract void traceComputationRows(
      Trace.Rlptxn trace, TransactionProcessingMetadata tx, GenericTracedValue tracedValues);

  protected abstract void traceIsPhaseX(Trace.Rlptxn trace);

  protected void traceLtLx(Trace.Rlptxn trace) {
    trace.lt(true).lx(true);
  }

  public void traceTransactionConstantValues(Trace.Rlptxn trace, GenericTracedValue tracedValues) {
    traceIsPhaseX(trace);
    trace
        .codeFragmentIndex(tracedValues.tx().getCodeFragmentIndex())
        .type0(tracedValues.type0())
        .type1(tracedValues.type1())
        .type2(tracedValues.type2())
        // .type3(tracedValues.type3())
        .type4(tracedValues.type4())
        .replayProtection(tracedValues.tx().replayProtection())
        .yParity(tracedValues.tx().yParity())
        .requiresEvmExecution(tracedValues.tx().requiresEvmExecution())
        .isDeployment(tracedValues.tx().isDeployment())
        .lengthOfDelegationList(tracedValues.tx().getBesuTransaction().codeDelegationListSize())
        .proverUserTxnNumberMax(tracedValues.userTxnNumberMax());
  }

  public void tracePostValues(Trace.Rlptxn trace, GenericTracedValue tracedValues) {
    trace
        .ltByteSizeCountdown(tracedValues.rlpLtByteSize())
        .lxByteSizeCountdown(tracedValues.rlpLxByteSize())
        .fillAndValidateRow();
  }

  public abstract int lineCount();
}
