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
import static net.consensys.linea.zktracer.Trace.RLP_UTILS_INST_DATA_PRICING;
import static net.consensys.linea.zktracer.module.rlpUtils.WcpExoCall.callToLeq;

import java.util.ArrayList;
import java.util.List;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlptxn.cancun.GenericTracedValue;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.types.Bytes16;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class InstructionDataPricing extends RlpUtilsCall {
  private static final Bytes32 TWO_FIFTY_FIVE = Bytes32.leftPad(Bytes.fromHexString("0xff"));
  @EqualsAndHashCode.Include @Getter private final Bytes16 limb;
  @EqualsAndHashCode.Include @Getter private final short nBytes;
  private final List<Short> zeros;
  private final List<Short> nonZeros;

  public InstructionDataPricing(Bytes16 limb, short nBytes) {
    super(nBytes - 1);
    this.limb = limb;
    this.nBytes = nBytes;
    zeros = new ArrayList<>(lineCount());
    nonZeros = new ArrayList<>(lineCount());
  }

  @Override
  protected void compute(Wcp wcp) {
    short numberZeros = 0;
    for (int ct = 0; ct < nBytes; ct++) {
      numberZeros += (short) (limb.get(ct) == (byte) 0 ? 1 : 0);
    }
    short numberNonZeros = (short) (nBytes - numberZeros);
    zeros.add(numberZeros);
    nonZeros.add(numberNonZeros);
    // call WCP to prove that the smallness of the byte
    for (int ct = 0; ct < nBytes; ct++) {
      final byte byteCT = limb.get(ct);
      final Bytes32 arg1 = Bytes32.leftPad(Bytes.of(byteCT));
      wcpCalls.add(callToLeq(wcp, arg1, TWO_FIFTY_FIVE));
      final boolean byteIsZero = byteCT == 0;
      numberZeros -= (short) (byteIsZero ? 1 : 0);
      numberNonZeros -= (short) (byteIsZero ? 0 : 1);
      zeros.add(numberZeros);
      nonZeros.add(numberNonZeros);
    }
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
        .pCmpExoData6(Bytes.ofUnsignedShort(zerosCount()))
        .pCmpExoData7(Bytes.ofUnsignedShort(nonZerosCount()))
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
        .macro(true)
        .pMacroInst(RLP_UTILS_INST_DATA_PRICING)
        .isDataPricing(true)
        .pMacroData1(limb)
        .pMacroData2(Bytes.ofUnsignedShort(nBytes))
        .pMacroData6(Bytes.ofUnsignedShort(zerosCount()))
        .pMacroData7(Bytes.ofUnsignedShort(nonZerosCount()))
        .pMacroData8(firstByte())
        .zeroCounter(zeros.getFirst())
        .nonzCounter(nonZeros.getFirst())
        .fillAndValidateRow();
  }

  @Override
  protected void traceCompt(Trace.Rlputils trace, short ct) {
    final boolean lastRow = ct == nBytes - 1;
    trace.compt(true).isDataPricing(true).ct(ct).ctMax(nBytes - 1);
    // related to WCP call
    wcpCalls.get(ct).traceWcpCall(trace);
    // byte decomposition of the limb
    trace
        .pComptLimb(limb)
        .pComptAcc(limb.slice(0, ct + 1))
        // call to POWER ref table for the last row
        .pComptShfFlag(lastRow)
        .pComptShfArg(lastRow ? LLARGE - nBytes : 0)
        .pComptShfPower(lastRow ? power(nBytes) : Bytes.EMPTY)
        // decrementing zeros and nonzeros counter
        .zeroCounter(zeros.get(ct + 1))
        .nonzCounter(nonZeros.get(ct + 1))
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

  @Override
  protected int computeLineCount() {
    return 1 + nBytes;
  } // 1 for MACRO and nBytes for CMPs

  private byte firstByte() {
    return limb.get(0);
  }

  public short zerosCount() {
    return zeros.getFirst();
  }

  public short nonZerosCount() {
    return nonZeros.getFirst();
  }
}
