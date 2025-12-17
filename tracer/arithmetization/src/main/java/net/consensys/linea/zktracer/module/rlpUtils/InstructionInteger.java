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
import static net.consensys.linea.zktracer.Trace.Rlptxn.RLP_TXN_CT_MAX_INTEGER;
import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.BI_PREFIX_SHORT_INT;
import static net.consensys.linea.zktracer.types.Utils.rightPadToBytes16;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class InstructionInteger extends RlpUtilsCall {
  @EqualsAndHashCode.Include @Getter private final Bytes32 integer;
  private boolean integerIsZero;
  private boolean integerHiIsNonZero;
  private boolean rlpPrefixRequired;

  public InstructionInteger(Bytes32 integer) {
    super();
    this.integer = integer;
  }

  @Override
  protected void compute() {
    integerIsZero = integer.isZero();
    integerHiIsNonZero = integer.numberOfLeadingZeroBytes() < LLARGE;
    final boolean intLoGeq128 = intLo().toUnsignedBigInteger().compareTo(BI_PREFIX_SHORT_INT) >= 0;
    rlpPrefixRequired = integerIsZero || integerHiIsNonZero || intLoGeq128;
  }

  @Override
  public void traceRlpTxn(
      Rlptxn trace,
      GenericTracedValue tracedValues,
      boolean lt,
      boolean lx,
      boolean updateTracedValue,
      int ct) {
    trace.cmp(true).ct(ct).ctMax(RLP_TXN_CT_MAX_INTEGER).lt(lt).lx(lx);

    switch (ct) {
      case 0 -> {
        // rlpUtils call:
        trace
            .pCmpRlputilsFlag(true)
            .pCmpRlputilsInst(RLP_UTILS_INST_INTEGER)
            .pCmpExoData1(intHi())
            .pCmpExoData2(intLo())
            .pCmpExoData3(!integerIsZero)
            .pCmpExoData4(integerHiIsNonZero)
            .pCmpExoData5(rlpPrefixRequired)
            .pCmpExoData6(rlpPrefix())
            .pCmpExoData7(leadingLimbShifted())
            .pCmpExoData8(leadingLimbByteSize());

        if (updateTracedValue && rlpPrefixRequired) {
          trace.limbConstructed(true).pCmpLimb(rlpPrefix()).pCmpLimbSize(1);
          if (lt) {
            tracedValues.decrementLtSizeBy(1);
          }
          if (lx) {
            tracedValues.decrementLxSizeBy(1);
          }
        }
      }

      case 1 -> {
        if (integerHiIsNonZero) {
          trace
              .limbConstructed(true)
              .pCmpLimb(leadingLimbShifted())
              .pCmpLimbSize(leadingLimbByteSize());
          if (lt && updateTracedValue) {
            tracedValues.decrementLtSizeBy(leadingLimbByteSize());
          }
          if (lx && updateTracedValue) {
            tracedValues.decrementLxSizeBy(leadingLimbByteSize());
          }
        }
      }

      case 2 -> {
        if (!integerIsZero) {
          final int limbLoSize = integerHiIsNonZero ? LLARGE : leadingLimbByteSize();
          trace
              .limbConstructed(true)
              .pCmpLimb(integerHiIsNonZero ? intLo() : leadingLimbShifted())
              .pCmpLimbSize(limbLoSize);
          if (lt & updateTracedValue) {
            tracedValues.decrementLtSizeBy(limbLoSize);
          }
          if (lx && updateTracedValue) {
            tracedValues.decrementLxSizeBy(limbLoSize);
          }
        }
      }
      default -> throw new IllegalArgumentException("Invalid counter: " + ct);
    }
  }

  @Override
  protected void traceMacro(Trace.Rlputils trace) {
    trace
        .inst(RLP_UTILS_INST_INTEGER)
        .data1(intHi())
        .data2(intLo())
        .data3(!integerIsZero)
        .data4(integerHiIsNonZero)
        .data5(rlpPrefixRequired)
        .data6(rlpPrefix())
        .data7(leadingLimbShifted())
        .data8(leadingLimbByteSize())
        .validateRow();
  }

  @Override
  protected short instruction() {
    return RLP_UTILS_INST_INTEGER;
  }

  @Override
  protected short compareTo(RlpUtilsCall other) {
    return (short)
        this.integer
            .toUnsignedBigInteger()
            .compareTo(((InstructionInteger) other).integer.toUnsignedBigInteger());
  }

  private Bytes intHi() {
    return integer.slice(0, LLARGE);
  }

  private Bytes intLo() {
    return integer.slice(LLARGE, LLARGE);
  }

  private Bytes leadingLimbShifted() {
    return rightPadToBytes16(leadingBytesNotShifted());
  }

  private int leadingLimbByteSize() {
    return leadingBytesNotShifted().size();
  }

  private Bytes leadingBytesNotShifted() {
    return (integerHiIsNonZero ? intHi() : intLo()).trimLeadingZeros();
  }

  private Bytes rlpPrefix() {
    return rlpPrefixRequired
        ? rightPadToBytes16(Bytes.of(RLP_PREFIX_INT_SHORT + integer.trimLeadingZeros().size()))
        : Bytes.EMPTY;
  }
}
