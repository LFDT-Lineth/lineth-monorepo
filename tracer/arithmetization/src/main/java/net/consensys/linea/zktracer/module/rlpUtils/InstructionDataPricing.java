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

import static graphql.com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.RLP_UTILS_INST_DATA_PRICING;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import org.apache.tuweni.bytes.Bytes;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class InstructionDataPricing extends RlpUtilsCall {
  @EqualsAndHashCode.Include @Getter private final Bytes limb;
  @EqualsAndHashCode.Include @Getter private final short nBytes;
  @Getter private Short zeros;
  @Getter private Short nonZeros;

  public InstructionDataPricing(Bytes limb, short nBytes) {
    super();
    checkArgument(limb.size() == LLARGE, "limb should be a Bytes16");
    this.limb = limb;
    this.nBytes = nBytes;
  }

  @Override
  protected void compute() {
    short numberZeros = 0;
    for (int ct = 0; ct < nBytes; ct++) {
      numberZeros += (short) (limb.get(ct) == (byte) 0 ? 1 : 0);
    }
    zeros = numberZeros;
    nonZeros = (short) (nBytes - zeros);
  }

  @Override
  public void traceRlpTxn(
      Trace.Rlptxn trace,
      GenericTracedValue tracedValues,
      boolean lt,
      boolean lx,
      boolean updateTracedValue,
      int ct) {
    trace
        .cmp(true)
        .ct(ct)
        .pCmpRlputilsFlag(true)
        .pCmpRlputilsInst(RLP_UTILS_INST_DATA_PRICING)
        .pCmpExoData1(limb)
        .pCmpExoData2(Bytes.ofUnsignedShort(nBytes))
        .pCmpExoData6(Bytes.ofUnsignedShort(zeros))
        .pCmpExoData7(Bytes.ofUnsignedShort(nonZeros))
        .pCmpExoData8(firstByte())
        .limbConstructed(true)
        .lt(true)
        .lx(true)
        .pCmpLimb(limb)
        .pCmpLimbSize(nBytes);

    if (updateTracedValue) {
      if (lt) {
        tracedValues.decrementLtSizeBy(nBytes);
      }
      if (lx) {
        tracedValues.decrementLxSizeBy(nBytes);
      }
    }
  }

  @Override
  protected void traceMacro(Trace.Rlputils trace) {
    trace
        .inst(RLP_UTILS_INST_DATA_PRICING)
        .data1(limb)
        .data2(Bytes.ofUnsignedShort(nBytes))
        .data6(Bytes.ofUnsignedShort(zeros))
        .data7(Bytes.ofUnsignedShort(nonZeros))
        .data8(firstByte())
        .fillAndValidateRow();
  }

  @Override
  protected short instruction() {
    return RLP_UTILS_INST_DATA_PRICING;
  }

  @Override
  protected short compareTo(RlpUtilsCall other) {
    final InstructionDataPricing o = (InstructionDataPricing) other;
    return (short) limb.slice(0, nBytes).compareTo((o.limb.slice(0, o.nBytes)));
  }

  private byte firstByte() {
    return limb.get(0);
  }
}
