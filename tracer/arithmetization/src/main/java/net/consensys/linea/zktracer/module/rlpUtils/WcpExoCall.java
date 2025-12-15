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

package net.consensys.linea.zktracer.module.rlpUtils;

import static net.consensys.linea.zktracer.Trace.*;

import lombok.Builder;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

@Builder
public class WcpExoCall {

  protected final int instruction;
  protected final Bytes32 arg1;
  protected final Bytes32 arg2;
  protected final boolean result;

  protected static WcpExoCall callToLeq(final Wcp wcp, Bytes arg1, Bytes arg2) {

    final Bytes32 arg1B32 = Bytes32.leftPad(arg1);
    final Bytes32 arg2B32 = Bytes32.leftPad(arg2);
    return WcpExoCall.builder()
        .instruction(WCP_INST_LEQ)
        .arg1(arg1B32)
        .arg2(arg2B32)
        .result(wcp.callLEQ(arg1B32, arg2B32))
        .build();
  }

  protected static WcpExoCall callToGeq(final Wcp wcp, Bytes32 arg1, Bytes32 arg2) {
    return WcpExoCall.builder()
        .instruction(WCP_INST_GEQ)
        .arg1(arg1)
        .arg2(arg2)
        .result(wcp.callGEQ(arg1, arg2))
        .build();
  }

  protected static WcpExoCall callToIsZero(final Wcp wcp, Bytes32 arg1) {
    return WcpExoCall.builder()
        .instruction(EVM_INST_ISZERO)
        .arg1(arg1)
        .result(wcp.callISZERO(arg1))
        .build();
  }

  protected static WcpExoCall callToGt(final Wcp wcp, Bytes32 arg1, Bytes32 arg2) {
    return WcpExoCall.builder()
        .instruction(EVM_INST_GT)
        .arg1(arg1)
        .arg2(arg2)
        .result(wcp.callGT(arg1, arg2))
        .build();
  }

  protected static WcpExoCall callToLt(final Wcp wcp, Bytes32 arg1, Bytes32 arg2) {
    return WcpExoCall.builder()
        .instruction(EVM_INST_LT)
        .arg1(arg1)
        .arg2(arg2)
        .result(wcp.callLT(arg1, arg2))
        .build();
  }

  protected static WcpExoCall callToEq(final Wcp wcp, Bytes32 arg1, Bytes32 arg2) {
    return WcpExoCall.builder()
        .instruction(EVM_INST_EQ)
        .arg1(arg1)
        .arg2(arg2)
        .result(wcp.callEQ(arg1, arg2))
        .build();
  }

  protected void traceWcpCall(Trace.Rlputils trace) {
    trace
        .pComptInst(instruction)
        .pComptArg1Hi(arg1.slice(0, LLARGE))
        .pComptArg1Lo(arg1.slice(LLARGE, LLARGE))
        .pComptArg2Lo(arg2 == null ? Bytes.EMPTY : arg2.slice(LLARGE, LLARGE))
        .pComptRes(result)
        .pComptWcpCtMax(wcpCtMax());
  }

  private long wcpCtMax() {
    if (instruction == EVM_INST_ISZERO || instruction == EVM_INST_EQ) {
      return 0;
    }
    final int nBytes =
        Math.max(
            Math.max(
                arg1.slice(0, LLARGE).trimLeadingZeros().size(),
                arg1.slice(LLARGE, LLARGE).trimLeadingZeros().size()),
            arg2.slice(LLARGE, LLARGE).trimLeadingZeros().size());

    return nBytes == 0 ? 0 : nBytes - 1;
  }
}
