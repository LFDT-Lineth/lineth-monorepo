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
import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.*;
import static net.consensys.linea.zktracer.types.Utils.BYTES16_ZERO;
import static net.consensys.linea.zktracer.types.Utils.rightPadToBytes16;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import org.apache.tuweni.bytes.Bytes;

@Accessors(fluent = true)
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public class InstructionByteStringPrefix extends RlpUtilsCall {

  // inputs
  @EqualsAndHashCode.Include @Getter private final int byteStringLength;
  @EqualsAndHashCode.Include @Getter private final byte firstByte;
  @EqualsAndHashCode.Include @Getter private final boolean isList;

  // outputs
  @Getter private boolean rlpPrefixRequired;
  private boolean byteStringIsNonEmpty;
  @Getter private Bytes rlpPrefix;
  @Getter private short rlpPrefixByteSize;

  public InstructionByteStringPrefix(int byteStringLength, byte firstByte, boolean isList) {
    super();
    this.byteStringLength = byteStringLength;
    this.firstByte = firstByte;
    this.isList = isList;
  }

  @Override
  protected void compute() {
    byteStringIsNonEmpty = byteStringLength != 0;
    final boolean byteStringLengthIsOne = byteStringLength == 1;
    final boolean byteStringLengthGtOne = byteStringLength > 1;

    if (!byteStringIsNonEmpty) {
      // setting output values
      rlpPrefixRequired = true;
      rlpPrefix = rightPadToBytes16(isList ? BYTES_PREFIX_SHORT_LIST : BYTES_PREFIX_SHORT_INT);
      rlpPrefixByteSize = 1;
    }

    if (byteStringLengthIsOne) {
      final boolean firstBytesLtPrefixShortInt = firstByte >= 0; // ie unsigned value is >= 128
      if (firstBytesLtPrefixShortInt) {
        rlpPrefixRequired = false;
        rlpPrefix = BYTES16_ZERO;
        rlpPrefixByteSize = 0;
      } else {
        rlpPrefixRequired = true;
        rlpPrefix =
            rightPadToBytes16(
                Bytes.minimalBytes(1 + (isList ? RLP_PREFIX_LIST_SHORT : RLP_PREFIX_INT_SHORT)));
        rlpPrefixByteSize = 1;
      }
    }

    if (byteStringLengthGtOne) {
      // computed values
      final boolean byteStringLengthGeq56 = byteStringLength >= 56;

      // setting outputs
      rlpPrefixRequired = true;
      if (!byteStringLengthGeq56) {
        rlpPrefix =
            rightPadToBytes16(
                Bytes.minimalBytes(
                    byteStringLength + (isList ? RLP_PREFIX_LIST_SHORT : RLP_PREFIX_INT_SHORT)));
        rlpPrefixByteSize = 1;
      } else {
        final int bslByteSize = Bytes.minimalBytes(byteStringLength).size();
        rlpPrefix =
            rightPadToBytes16(
                Bytes.concatenate(
                    Bytes.minimalBytes(
                        bslByteSize + (isList ? RLP_PREFIX_LIST_LONG : RLP_PREFIX_INT_LONG)),
                    Bytes.minimalBytes(byteStringLength)));
        rlpPrefixByteSize = (short) (1 + bslByteSize);
      }
    }
  }

  @Override
  public void traceRlpTxn(
      Rlptxn trace,
      GenericTracedValue tracedValues,
      boolean lt,
      boolean lx,
      boolean updateTracedValue,
      int ct) {
    trace
        .cmp(true)
        .lt(lt)
        .lx(lx)
        .pCmpRlputilsFlag(true)
        .pCmpRlputilsInst(RLP_UTILS_INST_BYTE_STRING_PREFIX)
        .pCmpExoData1(Bytes.ofUnsignedInt(byteStringLength))
        .pCmpExoData2(Bytes.of(firstByte))
        .pCmpExoData3(isList)
        .pCmpExoData4(byteStringIsNonEmpty)
        .pCmpExoData5(rlpPrefixRequired)
        .pCmpExoData6(rlpPrefix)
        .pCmpExoData8(rlpPrefixByteSize)
        .limbConstructed(rlpPrefixRequired)
        .pCmpLimb(rlpPrefix)
        .pCmpLimbSize(rlpPrefixByteSize);

    if (!updateTracedValue) {
      return;
    }

    if (rlpPrefixRequired && lt) {
      tracedValues.decrementLtSizeBy(rlpPrefixByteSize);
    }

    if (rlpPrefixRequired && lx) {
      tracedValues.decrementLxSizeBy(rlpPrefixByteSize);
    }
  }

  @Override
  protected void traceMacro(Trace.Rlputils trace) {
    trace
        .inst(RLP_UTILS_INST_BYTE_STRING_PREFIX)
        .data1(Bytes.ofUnsignedInt(byteStringLength))
        .data2(Bytes.of(firstByte))
        .data3(isList)
        .data4(byteStringIsNonEmpty)
        .data5(rlpPrefixRequired)
        .data6(rlpPrefix)
        .data8(rlpPrefixByteSize)
        .fillAndValidateRow();
  }

  @Override
  protected short instruction() {
    return RLP_UTILS_INST_BYTE_STRING_PREFIX;
  }

  @Override
  protected short compareTo(RlpUtilsCall other) {
    // first sort by byte string length
    final int byteStringLengthComparison =
        byteStringLength - (((InstructionByteStringPrefix) other).byteStringLength);

    if (byteStringLengthComparison != 0) {
      return (short) byteStringLengthComparison;
    }

    // then sort by first byte
    final int firstByteComparison =
        Byte.compare(firstByte, ((InstructionByteStringPrefix) other).firstByte);
    if (firstByteComparison != 0) {
      return (short) firstByteComparison;
    }

    return (short) (isList ? 1 : -1);
  }
}
