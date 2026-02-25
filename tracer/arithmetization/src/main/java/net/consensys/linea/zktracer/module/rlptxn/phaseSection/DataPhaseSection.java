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
import static net.consensys.linea.zktracer.Trace.LLARGEMO;
import static net.consensys.linea.zktracer.types.Utils.rightPadToBytes16;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlpUtils.InstructionByteStringPrefix;
import net.consensys.linea.zktracer.module.rlpUtils.InstructionDataPricing;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;

public class DataPhaseSection extends PhaseSection {
  private final InstructionByteStringPrefix prefix;
  private final List<InstructionDataPricing> limbs;

  public DataPhaseSection(RlpUtils rlpUtils, TransactionProcessingMetadata tx) {
    final Bytes data = tx.getBesuTransaction().getPayload();

    final InstructionByteStringPrefix prefixCall =
        new InstructionByteStringPrefix(
            data.size(), data.isEmpty() ? (byte) 0x00 : data.get(0), false);
    prefix = (InstructionByteStringPrefix) rlpUtils.call(prefixCall);

    final int dataSize = data.size();
    final int numberOfLimbs = (dataSize + LLARGEMO) / LLARGE; // Each limb is 16 bytes
    final int remaining = dataSize % LLARGE;
    final int nBytesLastLimb = remaining == 0 ? LLARGE : remaining;

    limbs = new ArrayList<>(numberOfLimbs);

    for (int i = 0; i < numberOfLimbs; i++) {
      final boolean lastLimb = i == numberOfLimbs - 1;
      final Bytes limbData =
          rightPadToBytes16(data.slice(i * LLARGE, lastLimb ? nBytesLastLimb : LLARGE));
      final InstructionDataPricing limbCall =
          new InstructionDataPricing(limbData, lastLimb ? (short) nBytesLastLimb : LLARGE);
      limbs.add((InstructionDataPricing) rlpUtils.call(limbCall));
    }
  }

  @Override
  protected void traceComputationRows(
      Trace.Rlptxn trace, TransactionProcessingMetadata tx, GenericTracedValue tracedValues) {
    // Trace the prefix
    traceTransactionConstantValues(trace, tracedValues);
    prefix.traceRlpTxn(trace, tracedValues, true, true, true, 0);
    trace.pCmpAux1(tx.numberOfZeroBytesInPayload()).pCmpAux2(tx.numberOfNonzeroBytesInPayload());
    tracePostValues(trace, tracedValues);

    // trace the limbs
    int zeros = tx.numberOfZeroBytesInPayload();
    int nonZeros = tx.numberOfNonzeroBytesInPayload();
    final int ctMax = limbs.size() - 1;
    for (int ct = 0; ct <= ctMax; ct++) {
      final InstructionDataPricing currentLimb = limbs.get(ct);
      traceTransactionConstantValues(trace, tracedValues);
      trace.ctMax(ctMax);
      currentLimb.traceRlpTxn(trace, tracedValues, true, true, true, ct);
      zeros -= currentLimb.zeros();
      nonZeros -= currentLimb.nonZeros();
      trace.pCmpAux1(zeros).pCmpAux2(nonZeros);
      tracePostValues(trace, tracedValues);
    }
  }

  @Override
  protected void traceIsPhaseX(Trace.Rlptxn trace) {
    trace.isData(true);
  }

  @Override
  public int lineCount() {
    return 1 + 1 + limbs.size();
  }
}
