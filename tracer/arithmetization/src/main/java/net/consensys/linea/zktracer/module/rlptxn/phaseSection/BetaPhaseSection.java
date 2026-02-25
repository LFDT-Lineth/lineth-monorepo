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

import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.BYTES_PREFIX_SHORT_INT;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes32;
import static net.consensys.linea.zktracer.types.Utils.rightPadToBytes16;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlpUtils.InstructionInteger;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

public class BetaPhaseSection extends PhaseSection {
  private final TransactionProcessingMetadata tx;
  private final InstructionInteger rlpTw;
  private InstructionInteger rlpBeta;

  public BetaPhaseSection(RlpUtils rlpUtils, TransactionProcessingMetadata tx) {
    this.tx = tx;

    final InstructionInteger call =
        new InstructionInteger(bigIntegerToBytes32(tx.getBesuTransaction().getV()));
    rlpTw = (InstructionInteger) rlpUtils.call(call);

    if (tx.replayProtection()) {
      final InstructionInteger call2 = new InstructionInteger(Bytes32.leftPad(tx.chainId()));
      rlpBeta = (InstructionInteger) rlpUtils.call(call2);
    }
  }

  @Override
  protected void traceComputationRows(
      Trace.Rlptxn trace, TransactionProcessingMetadata tx, GenericTracedValue tracedValues) {
    // trace rlpTw
    for (int ct = 0; ct <= 2; ct++) {
      traceTransactionConstantValues(trace, tracedValues);
      rlpTw.traceRlpTxn(trace, tracedValues, true, false, true, ct);
      tracePostValues(trace, tracedValues);
    }

    // we're done if !replayProtection
    if (tx.replayProtection()) {
      // trace rlpBeta
      for (int ct = 0; ct <= 2; ct++) {
        traceTransactionConstantValues(trace, tracedValues);
        rlpBeta.traceRlpTxn(trace, tracedValues, false, true, true, ct);
        tracePostValues(trace, tracedValues);
      }

      // trace rlp().rlp()
      traceTransactionConstantValues(trace, tracedValues);
      trace
          .cmp(true)
          .lx(true)
          .limbConstructed(true)
          .pCmpLimb(
              rightPadToBytes16(Bytes.concatenate(BYTES_PREFIX_SHORT_INT, BYTES_PREFIX_SHORT_INT)))
          .pCmpLimbSize(2);
      tracedValues.decrementLxSizeBy(2);
      tracePostValues(trace, tracedValues);
    }
  }

  @Override
  protected void traceLtLx(Trace.Rlptxn trace) {
    // nothing to trace
  }

  @Override
  protected void traceIsPhaseX(Trace.Rlptxn trace) {
    trace.isBeta(true);
  }

  @Override
  public int lineCount() {
    return 1 + 3 + (tx.replayProtection() ? 4 : 0);
  }
}
