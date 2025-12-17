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
package net.consensys.linea.zktracer.module.txndata.rows.computationRows;

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.*;

import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;

@RequiredArgsConstructor
@Accessors(fluent = true)
public class WcpRow extends ComputationRow {
  @Override
  public void traceRow(Trace.Txndata trace) {
    super.traceRow(trace);
    trace
        .pComputationWcpFlag(true)
        .pComputationArg1Lo(arg1)
        .pComputationArg2Lo(arg2)
        .pComputationInst(instruction.opCode)
        .pComputationWcpRes(result);
  }

  final WcpInstruction instruction;
  final Bytes arg1;
  final Bytes arg2;
  @Getter final boolean result;

  public static WcpRow smallCallToLt(Wcp wcp, final Bytes arg1, final Bytes arg2) {
    checkState(arg1.trimLeadingZeros().size() <= LLARGE, "arg1 = %s too large", arg1);
    checkState(arg2.trimLeadingZeros().size() <= LLARGE, "arg2 = %s too large", arg2);
    return new WcpRow(WcpInstruction.LT, arg1, arg2, wcp.callLT(arg1, arg2));
  }

  public static WcpRow smallCallToLt(Wcp wcp, final long arg1, final long arg2) {
    return new WcpRow(
        WcpInstruction.LT,
        Bytes.ofUnsignedLong(arg1),
        Bytes.ofUnsignedLong(arg2),
        wcp.callLT(arg1, arg2));
  }

  public static WcpRow smallCallToLeq(Wcp wcp, final Bytes arg1, final Bytes arg2) {
    return new WcpRow(WcpInstruction.LEQ, arg1, arg2, wcp.callLEQ(arg1, arg2));
  }

  public static WcpRow smallCallToLeq(Wcp wcp, final long arg1, final long arg2) {
    return new WcpRow(
        WcpInstruction.LEQ,
        Bytes.ofUnsignedLong(arg1),
        Bytes.ofUnsignedLong(arg2),
        wcp.callLEQ(arg1, arg2));
  }

  public static WcpRow smallCallToIszero(Wcp wcp, final long arg1) {
    final Bytes arg1Bytes = Bytes.ofUnsignedLong(arg1);
    return new WcpRow(WcpInstruction.ISZERO, arg1Bytes, Bytes.EMPTY, wcp.callISZERO(arg1Bytes));
  }

  @Accessors(fluent = true)
  private enum WcpInstruction {
    // EQ,
    LT(EVM_INST_LT),
    GT(EVM_INST_GT),
    // SLT,
    // SGT,
    ISZERO(EVM_INST_ISZERO),
    LEQ(WCP_INST_LEQ),
    GEQ(WCP_INST_GEQ);

    @Getter final int opCode;

    private WcpInstruction(int opCode) {
      this.opCode = opCode;
    }
  }
}
