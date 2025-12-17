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
import static net.consensys.linea.zktracer.Trace.Rlptxn.RLP_TXN_CT_MAX_ADDRESS;
import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.BYTES16_PREFIX_ADDRESS;
import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.BYTES_PREFIX_SHORT_INT;
import static net.consensys.linea.zktracer.types.AddressUtils.lowPart;
import static net.consensys.linea.zktracer.types.Utils.rightPadToBytes16;

import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.datatypes.Address;

public class ToPhaseSection extends PhaseSection {
  private final boolean isDeployment;

  public ToPhaseSection(TransactionProcessingMetadata tx) {
    isDeployment = tx.isDeployment();
  }

  @Override
  protected void traceComputationsRows(
      Trace.Rlptxn trace, TransactionProcessingMetadata tx, GenericTracedValue tracedValues) {
    if (tx.isDeployment()) {
      traceTransactionConstantValues(trace, tracedValues);
      trace
          .cmp(true)
          .limbConstructed(true)
          .lt(true)
          .lx(true)
          .pCmpLimb(rightPadToBytes16(BYTES_PREFIX_SHORT_INT))
          .pCmpLimbSize(1);
      tracedValues.decrementLtAndLxSizeBy(1);
      tracePostValues(trace, tracedValues);
    } else {
      final Address to = tx.getBesuTransaction().getTo().get();
      // Note: no need to call TRM module. The HUB will already trace the "To" account fragment and
      // call TRM. Calling it here would be a duplicate.

      // first row for deployment : rlp prefix
      traceTransactionConstantValues(trace, tracedValues);
      trace
          .cmp(true)
          .ctMax(RLP_TXN_CT_MAX_ADDRESS)
          .pCmpTrmFlag(true)
          .pCmpExoData1(to.slice(0, 4))
          .pCmpExoData2(lowPart(to))
          .limbConstructed(true)
          .lt(true)
          .lx(true)
          .pCmpLimb(BYTES16_PREFIX_ADDRESS)
          .pCmpLimbSize(1);
      tracedValues.decrementLtAndLxSizeBy(1);
      tracePostValues(trace, tracedValues);

      // second row for deployment : address hi
      traceTransactionConstantValues(trace, tracedValues);
      trace
          .cmp(true)
          .ct(1)
          .ctMax(RLP_TXN_CT_MAX_ADDRESS)
          .limbConstructed(true)
          .lt(true)
          .lx(true)
          .pCmpLimb(rightPadToBytes16(to.slice(0, 4)))
          .pCmpLimbSize(4);
      tracedValues.decrementLtAndLxSizeBy(4);
      tracePostValues(trace, tracedValues);

      // third row for deployment : address lo
      traceTransactionConstantValues(trace, tracedValues);
      trace
          .cmp(true)
          .ct(2)
          .ctMax(RLP_TXN_CT_MAX_ADDRESS)
          .limbConstructed(true)
          .lt(true)
          .lx(true)
          .pCmpLimb(to.slice(4, LLARGE))
          .pCmpLimbSize(LLARGE);
      tracedValues.decrementLtAndLxSizeBy(LLARGE);
      tracePostValues(trace, tracedValues);
    }
  }

  @Override
  protected void traceIsPhaseX(Trace.Rlptxn trace) {
    trace.isTo(true);
  }

  @Override
  public int lineCount() {
    return 1 + (isDeployment ? 1 : (RLP_TXN_CT_MAX_ADDRESS + 1));
  }
}
