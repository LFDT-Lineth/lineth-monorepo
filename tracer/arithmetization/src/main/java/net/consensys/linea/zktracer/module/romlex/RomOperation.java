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

package net.consensys.linea.zktracer.module.romlex;

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.types.Utils.rightPadTo;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.RequiredArgsConstructor;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.container.ModuleOperation;
import net.consensys.linea.zktracer.opcode.OpCodeData;
import net.consensys.linea.zktracer.opcode.OpCodes;
import net.consensys.linea.zktracer.types.UnsignedByte;
import org.apache.tuweni.bytes.Bytes;

@RequiredArgsConstructor
@Accessors(fluent = true)
@Getter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = false)
public final class RomOperation extends ModuleOperation {
  private static final UnsignedByte UB_LLARGE = UnsignedByte.of(LLARGE);
  private static final UnsignedByte UB_LLARGE_MO = UnsignedByte.of(LLARGEMO);
  private static final UnsignedByte UB_EVW_WORD_MO = UnsignedByte.of(WORD_SIZE_MO);

  @Getter @EqualsAndHashCode.Include private final ContractMetadata metadata;
  private final Bytes byteCode;

  private final OpCodes opCodes;

  public void trace(Trace.Rom trace, int cfi, int cfiInfty) {
    // WARN this is the tracing used by the ROM, not by the ROMLEX
    final int chunkRowSize = this.lineCount();
    final int codeSize = this.byteCode().size();
    final int nLimbSlice = (codeSize + (LLARGE - 1)) / LLARGE;
    final Bytes dataPadded = rightPadTo(this.byteCode(), chunkRowSize);
    int nBytesLastRow = codeSize % LLARGE;
    if (nBytesLastRow == 0) {
      nBytesLastRow = LLARGE;
    }

    int pushParameter = 0;
    int ctPush = 0;
    Bytes pushValueHigh = Bytes.minimalBytes(0);
    Bytes pushValueLow = Bytes.minimalBytes(0);

    for (int i = 0; i < chunkRowSize; i++) {
      final boolean codeSizeReached = i >= codeSize;
      final int sliceNumber = i / LLARGE;

      // Fill Generic columns
      trace
          .codeFragmentIndex(cfi)
          .codeFragmentIndexInfty(cfiInfty)
          .programCounter(i)
          .limb(dataPadded.slice(sliceNumber * LLARGE, LLARGE))
          .codeSize(codeSize)
          .paddedBytecodeByte(UnsignedByte.of(dataPadded.get(i)))
          .acc(dataPadded.slice(sliceNumber * LLARGE, (i % LLARGE) + 1))
          .codesizeReached(codeSizeReached)
          .index(sliceNumber);

      // Fill CT, CTmax nBYTES, nBYTES_ACC
      if (sliceNumber < nLimbSlice) {
        trace.counter(UnsignedByte.of(i % LLARGE)).counterMax(UB_LLARGE_MO);
        if (sliceNumber < nLimbSlice - 1) {
          trace.nBytes(UB_LLARGE).nBytesAcc(UnsignedByte.of((i % LLARGE) + 1));
        }
        if (sliceNumber == nLimbSlice - 1) {
          trace
              .nBytes(UnsignedByte.of(nBytesLastRow))
              .nBytesAcc(UnsignedByte.of(Math.min(nBytesLastRow, (i % LLARGE) + 1)));
        }
      } else if (sliceNumber == nLimbSlice || sliceNumber == nLimbSlice + 1) {
        trace
            .counter(UnsignedByte.of(i - nLimbSlice * LLARGE))
            .counterMax(UB_EVW_WORD_MO)
            .nBytes(UnsignedByte.ZERO)
            .nBytesAcc(UnsignedByte.ZERO);
      }

      // Deal when not in a PUSH instruction
      if (pushParameter == 0) {
        final UnsignedByte opCodeUB = UnsignedByte.of(dataPadded.get(i));
        final OpCodeData opCode = opCodes.of(opCodeUB.toInteger());
        final boolean isPush = opCode.isNonTrivialPush();

        // The OpCode is a PUSH instruction
        if (isPush) {
          pushParameter = opCodeUB.toInteger() - EVM_INST_PUSH0;
          if (pushParameter > LLARGE) {
            pushValueHigh = dataPadded.slice(i + 1, pushParameter - LLARGE);
            pushValueLow = dataPadded.slice(i + 1 + pushParameter - LLARGE, LLARGE);
          } else {
            pushValueLow = dataPadded.slice(i + 1, pushParameter);
          }
        }

        trace
            .isPush(isPush)
            .isPushData(false)
            .opcode(opCodeUB)
            .pushParameter(UnsignedByte.of(pushParameter))
            .counterPush(UnsignedByte.ZERO)
            .pushValueAcc(Bytes.EMPTY)
            .pushValueHi(pushValueHigh)
            .pushValueLo(pushValueLow)
            .pushFunnelBit(false)
            .isJumpdest(opCode.isJumpDest());
      }
      // Deal when in a PUSH instruction
      else {
        ctPush += 1;
        trace
            .isPush(false)
            .isPushData(true)
            .opcode(UnsignedByte.of(EVM_INST_INVALID))
            .pushParameter(UnsignedByte.of(pushParameter))
            .pushValueHi(pushValueHigh)
            .pushValueLo(pushValueLow)
            .counterPush(UnsignedByte.of(ctPush))
            .pushFunnelBit(pushParameter > LLARGE && ctPush > pushParameter - LLARGE)
            .isJumpdest(false);

        if (pushParameter <= LLARGE) {
          trace.pushValueAcc(pushValueLow.slice(0, ctPush));
        } else {
          if (ctPush <= pushParameter - LLARGE) {
            trace.pushValueAcc(pushValueHigh.slice(0, ctPush));
          } else {
            trace.pushValueAcc(pushValueLow.slice(0, ctPush + LLARGE - pushParameter));
          }
        }

        // reinitialise push constant data
        if (ctPush == pushParameter) {
          ctPush = 0;
          pushParameter = 0;
          pushValueHigh = Bytes.minimalBytes(0);
          pushValueLow = Bytes.minimalBytes(0);
        }
      }

      trace.validateRow();
    }
  }

  @Override
  protected int computeLineCount() {
    // WARN this is the line count used by the ROM, not by the ROMLEX
    final int nPaddingRow = WORD_SIZE;
    final int codeSize = this.byteCode.size();
    final int nbSlice = (codeSize + (LLARGE - 1)) / LLARGE;

    return LLARGE * nbSlice + nPaddingRow;
  }
}
