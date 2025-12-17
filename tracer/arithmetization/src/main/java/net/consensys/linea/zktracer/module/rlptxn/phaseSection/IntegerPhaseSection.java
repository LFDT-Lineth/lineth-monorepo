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

import static net.consensys.linea.zktracer.module.rlptxn.phaseSection.IntegerEntry.*;
import static net.consensys.linea.zktracer.types.Conversions.*;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlpUtils.InstructionInteger;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtilsCall;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes32;

public class IntegerPhaseSection extends PhaseSection {

  private final IntegerEntry entry;
  private final boolean lx;
  private final InstructionInteger intCall;

  public IntegerPhaseSection(
      RlpUtils rlpUtils, IntegerEntry entry, TransactionProcessingMetadata tx) {
    lx = entry.lx();
    this.entry = entry;
    final Bytes32 integer =
        switch (entry) {
          case CHAIN_ID -> Bytes32.leftPad(tx.chainId());
          case NONCE -> longToBytes32(tx.getBesuTransaction().getNonce());
          case GAS_PRICE -> Bytes32.leftPad(tx.gasPrice());
          case MAX_PRIORITY_FEE_PER_GAS -> Bytes32.leftPad(tx.maxPriorityFeePerGas());
          case MAX_FEE_PER_GAS -> Bytes32.leftPad(tx.maxFeePerGas());
          case GAS_LIMIT -> longToBytes32(tx.getBesuTransaction().getGasLimit());
          case VALUE -> bigIntegerToBytes32(tx.getBesuTransaction().getValue().getAsBigInteger());
          case Y -> Bytes32.leftPad(booleanToBytes(tx.yParity()));
          case R -> bigIntegerToBytes32(tx.getBesuTransaction().getR());
          case S -> bigIntegerToBytes32(tx.getBesuTransaction().getS());
        };

    final RlpUtilsCall call = new InstructionInteger(integer);
    intCall = (InstructionInteger) rlpUtils.call(call);
  }

  @Override
  protected void traceComputationsRows(
      Trace.Rlptxn trace, TransactionProcessingMetadata tx, GenericTracedValue tracedValues) {
    for (int ct = 0; ct <= 2; ct++) {
      traceTransactionConstantValues(trace, tracedValues);
      intCall.traceRlpTxn(trace, tracedValues, true, lx, true, ct);
      tracePostValues(trace, tracedValues);
    }
  }

  @Override
  protected void traceIsPhaseX(Trace.Rlptxn trace) {
    trace
        .isChainId(entry == CHAIN_ID)
        .isNonce(entry == NONCE)
        .isGasPrice(entry == GAS_PRICE)
        .isMaxPriorityFeePerGas(entry == MAX_PRIORITY_FEE_PER_GAS)
        .isMaxFeePerGas(entry == MAX_FEE_PER_GAS)
        .isGasLimit(entry == GAS_LIMIT)
        .isValue(entry == VALUE)
        .isY(entry == Y)
        .isR(entry == R)
        .isS(entry == S);
  }

  @Override
  protected void traceLtLx(Trace.Rlptxn trace) {
    trace.lt(true).lx(lx);
  }

  @Override
  public int lineCount() {
    return 4; // 1 for the txn, 3 for the computation of the rlp
  }
}
