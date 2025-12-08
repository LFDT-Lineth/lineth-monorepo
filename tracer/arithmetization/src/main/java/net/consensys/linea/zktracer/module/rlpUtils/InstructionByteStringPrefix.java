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
import static net.consensys.linea.zktracer.Trace.Rlputils.CT_MAX_INST_BYTE_STRING_PREFIX;
import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.*;
import static net.consensys.linea.zktracer.types.Conversions.bytesToLong;

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
public class InstructionByteStringPrefix extends RlpUtilsCall {

  private static final Bytes32 ONE = Bytes32.leftPad(Bytes.of(1));

  // inputs
  @EqualsAndHashCode.Include @Getter private final Bytes32 byteStringLength;
  @EqualsAndHashCode.Include @Getter private final byte firstByte;
  @EqualsAndHashCode.Include @Getter private final boolean isList;

  // outputs
  @Getter private boolean rlpPrefixRequired;
  private boolean byteStringIsNonEmpty;
  @Getter private Bytes16 rlpPrefix;
  @Getter private short rlpPrefixByteSize;

  // computed values
  private boolean byteStringLengthGeq56 = false;
  private int bslByteSize = 1; // default value, will be updated if needed

  public InstructionByteStringPrefix(int byteStringLength, byte firstByte, boolean isList) {
    super(CT_MAX_INST_BYTE_STRING_PREFIX);
    this.byteStringLength = Bytes32.leftPad(Bytes.ofUnsignedInt(byteStringLength));
    this.firstByte = firstByte;
    this.isList = isList;
  }

  @Override
  protected void compute(Wcp wcp) {
    final WcpExoCall firstCall = WcpExoCall.callToIsZero(wcp, byteStringLength);
    wcpCalls.add(firstCall);
    byteStringIsNonEmpty = !firstCall.result;

    final WcpExoCall secondCall = WcpExoCall.callToEq(wcp, byteStringLength, ONE);
    wcpCalls.add(secondCall);
    final boolean byteStringLengthIsOne = byteStringIsNonEmpty && secondCall.result;
    final boolean byteStringLengthGtOne = byteStringIsNonEmpty && !secondCall.result;

    if (!byteStringIsNonEmpty) {
      // no constraints, we just add a stupid wcp call to not break the lookup
      wcpCalls.add(firstCall);
      // setting output values
      rlpPrefixRequired = true;
      rlpPrefix = Bytes16.rightPad(isList ? BYTES_PREFIX_SHORT_LIST : BYTES_PREFIX_SHORT_INT);
      rlpPrefixByteSize = 1;
    }

    if (byteStringLengthIsOne) {
      final WcpExoCall thirdCall =
          WcpExoCall.callToLt(wcp, Bytes32.leftPad(Bytes.of(firstByte)), BYTES32_PREFIX_SHORT_INT);
      wcpCalls.add(thirdCall);

      final boolean firstBytesLtPrefixShortInt = thirdCall.result;
      if (firstBytesLtPrefixShortInt) {
        rlpPrefixRequired = false;
        rlpPrefix = Bytes16.ZERO;
        rlpPrefixByteSize = 0;
      } else {
        rlpPrefixRequired = true;
        rlpPrefix =
            Bytes16.rightPad(
                Bytes.minimalBytes(1 + (isList ? RLP_PREFIX_LIST_SHORT : RLP_PREFIX_INT_SHORT)));
        rlpPrefixByteSize = 1;
      }
    }

    if (byteStringLengthGtOne) {
      final WcpExoCall thirdCall =
          WcpExoCall.callToLt(wcp, byteStringLength, Bytes32.leftPad(Bytes.minimalBytes(56)));
      wcpCalls.add(thirdCall);
      byteStringLengthGeq56 = !thirdCall.result;

      // setting outputs
      rlpPrefixRequired = true;
      if (!byteStringLengthGeq56) {
        rlpPrefix =
            Bytes16.rightPad(
                Bytes.minimalBytes(
                    bytesToLong(byteStringLength)
                        + (isList ? RLP_PREFIX_LIST_SHORT : RLP_PREFIX_INT_SHORT)));
        rlpPrefixByteSize = 1;
      } else {
        bslByteSize = byteStringLength.trimLeadingZeros().size();
        rlpPrefix =
            Bytes16.rightPad(
                Bytes.concatenate(
                    Bytes.minimalBytes(
                        bslByteSize + (isList ? RLP_PREFIX_LIST_LONG : RLP_PREFIX_INT_LONG)),
                    byteStringLength.trimLeadingZeros()));
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
        .pCmpExoData1(byteStringLength)
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
        .macro(true)
        .pMacroInst(RLP_UTILS_INST_BYTE_STRING_PREFIX)
        .isByteStringPrefix(true)
        .pMacroData1(byteStringLength)
        .pMacroData2(Bytes.of(firstByte))
        .pMacroData3(isList)
        .pMacroData4(byteStringIsNonEmpty)
        .pMacroData5(rlpPrefixRequired)
        .pMacroData6(rlpPrefix)
        .pMacroData8(rlpPrefixByteSize)
        .fillAndValidateRow();
  }

  @Override
  protected void traceCompt(Trace.Rlputils trace, short ct) {
    final boolean lastRow = ct == CT_MAX_INST_BYTE_STRING_PREFIX;
    trace.compt(true).isByteStringPrefix(true).ct(ct).ctMax(CT_MAX_INST_BYTE_STRING_PREFIX);
    // related to WCP call
    wcpCalls.get(ct).traceWcpCall(trace);
    // call to POWER ref table for the last row
    if (byteStringLengthGeq56 && lastRow) {
      trace
          .pComptShfFlag(true)
          .pComptShfArg(LLARGE - (bslByteSize + 1))
          .pComptShfPower(power(bslByteSize + 1));
    }
    trace.fillAndValidateRow();
  }

  @Override
  protected short instruction() {
    return RLP_UTILS_INST_BYTE_STRING_PREFIX;
  }

  @Override
  protected short compareTo(RlpUtilsCall other) {
    // first sort by byte string length
    final int byteStringLengthComparison =
        byteStringLength.compareTo(((InstructionByteStringPrefix) other).byteStringLength);

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

  @Override
  protected int computeLineCount() {
    return 1 + CT_MAX_INST_BYTE_STRING_PREFIX + 1;
  }
}
