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

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.RLP_UTILS_INST_BYTES32;
import static net.consensys.linea.zktracer.Trace.Rlptxn.RLP_TXN_CT_MAX_BYTES32;
import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.BYTES16_PREFIX_BYTES32;

import lombok.EqualsAndHashCode;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class InstructionBytes32 extends RlpUtilsCall {
  @EqualsAndHashCode.Include private final Bytes32 input1;

  public InstructionBytes32(Bytes32 input1) {
    super();
    this.input1 = input1;
  }

  @Override
  protected void compute() {}

  @Override
  public void traceRlpTxn(
      Trace.Rlptxn trace,
      GenericTracedValue tracedValues,
      boolean lt,
      boolean lx,
      boolean updateTracedValue,
      int ct) {
    trace.cmp(true).isAccessListStorageKey(true);
    tracedValues.decrementLtAndLxSizeBy(ct == 0 ? 1 : LLARGE);
    trace.limbConstructed(true).lt(true).lx(true).ct(ct).ctMax(RLP_TXN_CT_MAX_BYTES32);
    switch (ct) {
      case 0 ->
          trace
              .pCmpRlputilsFlag(true)
              .pCmpRlputilsInst(RLP_UTILS_INST_BYTES32)
              .pCmpExoData1(data1())
              .pCmpExoData2(data2())
              .pCmpLimb(BYTES16_PREFIX_BYTES32)
              .pCmpLimbSize(1);
      case 1 -> trace.pCmpLimb(data1()).pCmpLimbSize(LLARGE);
      case 2 -> trace.pCmpLimb(data2()).pCmpLimbSize(LLARGE);
      default -> throw new IllegalArgumentException("Invalid counter: " + ct);
    }
  }

  @Override
  protected void traceMacro(Trace.Rlputils trace) {
    trace.inst(RLP_UTILS_INST_BYTES32).data1(data1()).data2(data2()).fillAndValidateRow();
  }

  @Override
  protected short instruction() {
    return RLP_UTILS_INST_BYTES32;
  }

  @Override
  protected short compareTo(RlpUtilsCall other) {
    return (short)
        input1
            .toUnsignedBigInteger()
            .compareTo(((InstructionBytes32) other).input1.toUnsignedBigInteger());
  }

  private Bytes data1() {
    return input1.slice(0, LLARGE);
  }

  private Bytes data2() {
    return input1.slice(LLARGE, LLARGE);
  }
}
