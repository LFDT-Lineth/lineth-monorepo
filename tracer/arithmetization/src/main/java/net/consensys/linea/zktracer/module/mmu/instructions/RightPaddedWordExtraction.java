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

package net.consensys.linea.zktracer.module.mmu.instructions;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.LLARGEPO;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_VANISHES;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_ONE_SOURCE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_MICRO_ROWS_TOT_RIGHT_PADDED_WORD_EXTRACTION;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.types.Conversions.*;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;

import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.euc.EucOperation;
import net.consensys.linea.zktracer.module.mmu.MmuData;
import net.consensys.linea.zktracer.module.mmu.values.HubToMmuValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuEucCallRecord;
import net.consensys.linea.zktracer.module.mmu.values.MmuOutAndBinValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuToMmioConstantValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuToMmioInstruction;
import net.consensys.linea.zktracer.module.mmu.values.MmuWcpCallRecord;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import org.apache.commons.lang3.BooleanUtils;
import org.apache.tuweni.bytes.Bytes;

public class RightPaddedWordExtraction implements MmuInstruction {
  private final Euc euc;
  private final Wcp wcp;
  private final List<MmuEucCallRecord> eucCallRecords;
  private final List<MmuWcpCallRecord> wcpCallRecords;

  private long totalSourceOffset;
  private short firstLimbByteSize;
  private boolean secondLimbPadded;
  private short secondLimbByteSize;
  private short extractionSize;
  private boolean firstLimbIsFull;
  private boolean firstLimbSingleSource;
  private boolean secondLimbSingleSource;
  private Bytes sourceLimbOffset;
  private Bytes sourceByteOffset;
  private boolean secondLimbVoid;

  public RightPaddedWordExtraction(Euc euc, Wcp wcp) {
    this.euc = euc;
    this.wcp = wcp;
    this.eucCallRecords = new ArrayList<>(NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION);
    this.wcpCallRecords = new ArrayList<>(NB_PP_ROWS_RIGHT_PADDED_WORD_EXTRACTION);
  }

  @Override
  public MmuData preProcess(MmuData mmuData) {
    final HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();
    row1(hubToMmuValues);
    row2();
    row3(hubToMmuValues);
    row4();
    row5();

    mmuData.eucCallRecords(eucCallRecords);
    mmuData.wcpCallRecords(wcpCallRecords);

    // setting Out and Bin values
    mmuData.outAndBinValues(MmuOutAndBinValues.DEFAULT); // all 0

    mmuData.totalLeftZeroesInitials(0);
    mmuData.totalNonTrivialInitials(NB_MICRO_ROWS_TOT_RIGHT_PADDED_WORD_EXTRACTION);
    mmuData.totalRightZeroesInitials(0);

    return mmuData;
  }

  private void row1(final HubToMmuValues hubToMmuValues) {
    // row n°1
    final Bytes wcpArg1 =
        bigIntegerToBytes(hubToMmuValues.sourceOffsetLo().add(BigInteger.valueOf(32)));
    final long refSize = hubToMmuValues.referenceSize();
    final Bytes wcpArg2 = longToBytes(refSize);
    final boolean wcpResult = wcp.callLT(wcpArg1, wcpArg2);
    secondLimbPadded = !wcpResult;
    extractionSize =
        (short)
            (secondLimbPadded ? refSize - hubToMmuValues.sourceOffsetLo().longValue() : WORD_SIZE);

    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(wcpResult).build());

    eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
  }

  private void row2() {
    // row n°2
    final Bytes wcpArg1 = longToBytes(extractionSize);
    final Bytes wcpArg2 = Bytes.of(LLARGE);
    final boolean firstLimbPadded = wcp.callLT(wcpArg1, wcpArg2);

    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder()
            .arg1Lo(wcpArg1)
            .arg2Lo(wcpArg2)
            .result(firstLimbPadded)
            .build());

    if (!secondLimbPadded) {
      secondLimbByteSize = LLARGE;
    } else {
      secondLimbByteSize = (short) (!firstLimbPadded ? (extractionSize - LLARGE) : 0);
    }

    firstLimbByteSize = !firstLimbPadded ? LLARGE : extractionSize;

    final Bytes dividend = Bytes.of(firstLimbByteSize);
    final EucOperation eucOp = euc.callEUC(dividend, Bytes.of(LLARGE));

    firstLimbIsFull = BooleanUtils.toBoolean(eucOp.quotient().toInt());

    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend.toLong())
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());
  }

  private void row3(final HubToMmuValues hubToMmuValues) {
    // row n°3
    totalSourceOffset =
        hubToMmuValues.sourceOffsetLo().longValue() + hubToMmuValues.referenceOffset();
    final EucOperation eucOp = euc.callEUC(longToBytes(totalSourceOffset), Bytes.of(LLARGE));

    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(totalSourceOffset)
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());

    sourceLimbOffset = eucOp.quotient();
    sourceByteOffset = eucOp.remainder();

    final Bytes wcpArg1 = Bytes.ofUnsignedShort(sourceByteOffset.toInt() + firstLimbByteSize);
    final Bytes wcpArg2 = Bytes.of(LLARGEPO);
    firstLimbSingleSource = wcp.callLT(wcpArg1, wcpArg2);

    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder()
            .arg1Lo(wcpArg1)
            .arg2Lo(wcpArg2)
            .result(firstLimbSingleSource)
            .build());
  }

  private void row4() {
    // row n°4
    eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);

    final Bytes wcpArg1 = Bytes.ofUnsignedShort(sourceByteOffset.toInt() + secondLimbByteSize);
    final Bytes wcpArg2 = Bytes.of(LLARGEPO);
    secondLimbSingleSource = wcp.callLT(wcpArg1, wcpArg2);

    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder()
            .arg1Lo(wcpArg1)
            .arg2Lo(wcpArg2)
            .result(secondLimbSingleSource)
            .build());
  }

  private void row5() {
    // row n°5
    eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);

    final Bytes isZeroArg = Bytes.ofUnsignedShort(secondLimbByteSize);
    boolean wcpResult = wcp.callISZERO(isZeroArg);

    wcpCallRecords.add(
        MmuWcpCallRecord.instIsZeroBuilder().arg1Lo(isZeroArg).result(wcpResult).build());

    secondLimbVoid = wcpResult;
  }

  @Override
  public MmuData setMicroInstructions(MmuData mmuData) {
    final HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    // Setting MMIO constant values
    mmuData.mmuToMmioConstantValues(
        MmuToMmioConstantValues.builder().sourceContextNumber(hubToMmuValues.sourceId()).build());

    // Setting the list of MMIO instructions
    firstMicroInstruction(mmuData);
    secondMicroInstruction(mmuData);

    return mmuData;
  }

  private void firstMicroInstruction(MmuData mmuData) {
    int firstMicroInst;
    if (firstLimbSingleSource) {
      firstMicroInst =
          firstLimbIsFull ? MMIO_INST_RAM_TO_LIMB_TRANSPLANT : MMIO_INST_RAM_TO_LIMB_ONE_SOURCE;
    } else {
      firstMicroInst = MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
    }

    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(firstMicroInst)
            .size(firstLimbByteSize)
            .sourceLimbOffset(sourceLimbOffset.toInt())
            .sourceByteOffset((short) sourceByteOffset.toInt())
            .limb(mmuData.hubToMmuValues().limb1())
            .build());
  }

  private void secondMicroInstruction(MmuData mmuData) {
    int secondMicroInst;
    if (secondLimbVoid) {
      secondMicroInst = MMIO_INST_LIMB_VANISHES;
    } else {
      if (secondLimbSingleSource) {
        secondMicroInst =
            secondLimbPadded ? MMIO_INST_RAM_TO_LIMB_ONE_SOURCE : MMIO_INST_RAM_TO_LIMB_TRANSPLANT;
      } else {
        secondMicroInst = MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
      }
    }

    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(secondMicroInst)
            .size(secondLimbByteSize)
            .sourceLimbOffset(sourceLimbOffset.toInt() + 1)
            .sourceByteOffset((short) sourceByteOffset.toInt())
            .limb(mmuData.hubToMmuValues().limb2())
            .build());
  }
}
