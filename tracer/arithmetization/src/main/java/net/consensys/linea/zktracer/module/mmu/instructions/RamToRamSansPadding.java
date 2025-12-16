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
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_PARTIAL;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_TWO_SOURCE;
import static net.consensys.linea.zktracer.Trace.Mmu.NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.longToBytes;

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
import org.apache.tuweni.bytes.Bytes;

public class RamToRamSansPadding implements MmuInstruction {
  private final Euc euc;
  private final Wcp wcp;
  private final List<MmuEucCallRecord> eucCallRecords;
  private final List<MmuWcpCallRecord> wcpCallRecords;
  private short lastLimbByteSize;
  private short middleSourceByteOffset;
  private boolean lastLimbSingleSource;
  private boolean initialSloIncrement;
  private boolean lastLimbIsFast;
  private long initialSourceLimbOffset;
  private short initialSourceByteOffset;
  private long initialTargetLimbOffset;
  private short initialTargetByteOffset;
  private long realSize;
  private boolean aligned;
  private long finalTargetLimbOffset;
  private long totInitialNonTrivial;
  private boolean totNonTrivialIsOne;
  private short firstLimbByteSize;
  private boolean firstLimbSingleSource;
  private boolean initialTboIsZero;
  private boolean lastLimbIsFull;
  private boolean firstLimbIsFast;

  public RamToRamSansPadding(Euc euc, Wcp wcp) {
    this.euc = euc;
    this.wcp = wcp;
    this.eucCallRecords = new ArrayList<>(NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING);
    this.wcpCallRecords = new ArrayList<>(NB_PP_ROWS_RAM_TO_RAM_SANS_PADDING);
  }

  @Override
  public MmuData preProcess(final MmuData mmuData) {
    final HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();
    row1(hubToMmuValues);
    row2(hubToMmuValues);
    row3(hubToMmuValues);
    row4();
    row5();

    mmuData.eucCallRecords(eucCallRecords);
    mmuData.wcpCallRecords(wcpCallRecords);

    // setting Out and Bin values
    mmuData.outAndBinValues(
        MmuOutAndBinValues.builder()
            .out1(lastLimbByteSize)
            .out2(middleSourceByteOffset)
            .bin1(aligned)
            .bin2(lastLimbSingleSource)
            .bin3(initialSloIncrement)
            .bin4(lastLimbIsFast)
            .build());

    mmuData.totalLeftZeroesInitials(0);
    mmuData.totalNonTrivialInitials((int) totInitialNonTrivial);
    mmuData.totalRightZeroesInitials(0);

    return mmuData;
  }

  private void row1(final HubToMmuValues hubToMmuValues) {
    // row n°1
    final Bytes dividend = bigIntegerToBytes(hubToMmuValues.sourceOffsetLo());
    final EucOperation eucOp = euc.callEUC(dividend, Bytes.of(LLARGE));

    initialSourceLimbOffset = eucOp.quotient().toLong();
    initialSourceByteOffset = (short) eucOp.remainder().toInt();

    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend.toLong())
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());

    final Bytes wcpArg1 = longToBytes(hubToMmuValues.referenceSize());
    final Bytes wcpArg2 = longToBytes(hubToMmuValues.size());
    final boolean wcpResult = wcp.callLT(wcpArg1, wcpArg2);

    realSize = wcpResult ? hubToMmuValues.referenceSize() : hubToMmuValues.size();

    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(wcpResult).build());
  }

  private void row2(final HubToMmuValues hubToMmuValues) {
    // row n°2
    final Bytes dividend = longToBytes(hubToMmuValues.referenceOffset());
    final EucOperation eucOp = euc.callEUC(dividend, Bytes.of(LLARGE));

    initialTargetLimbOffset = eucOp.quotient().toLong();
    initialTargetByteOffset = (short) eucOp.remainder().toInt();

    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend.toLong())
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());

    final Bytes wcpArg1 = longToBytes(initialSourceByteOffset);
    final Bytes wcpArg2 = longToBytes(initialTargetByteOffset);
    aligned = wcp.callEQ(wcpArg1, wcpArg2);

    wcpCallRecords.add(
        MmuWcpCallRecord.instEqBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(aligned).build());
  }

  private void row3(final HubToMmuValues hubToMmuValues) {
    // row n°3
    final Bytes dividend = longToBytes(hubToMmuValues.referenceOffset() + realSize - 1);
    final EucOperation eucOp = euc.callEUC(dividend, Bytes.of(LLARGE));

    finalTargetLimbOffset = eucOp.quotient().toLong();

    totInitialNonTrivial = finalTargetLimbOffset - initialTargetLimbOffset + 1;

    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend.toLong())
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());

    final Bytes wcpArg1 = longToBytes(totInitialNonTrivial);
    final Bytes wcpArg2 = Bytes.of(1);
    totNonTrivialIsOne = wcp.callEQ(wcpArg1, wcpArg2);

    wcpCallRecords.add(
        MmuWcpCallRecord.instEqBuilder()
            .arg1Lo(wcpArg1)
            .arg2Lo(wcpArg2)
            .result(totNonTrivialIsOne)
            .build());

    firstLimbByteSize =
        (short) (totNonTrivialIsOne ? (int) realSize : LLARGE - initialTargetByteOffset);
    lastLimbByteSize = (short) (totNonTrivialIsOne ? realSize : 1 + eucOp.remainder().toInt());
  }

  private void row4() {
    // row n°4
    final Bytes wcpArg1 = longToBytes(initialSourceByteOffset + firstLimbByteSize - 1);
    final Bytes wcpArg2 = Bytes.of(LLARGE);
    firstLimbSingleSource = wcp.callLT(wcpArg1, wcpArg2);
    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder()
            .arg1Lo(wcpArg1)
            .arg2Lo(wcpArg2)
            .result(firstLimbSingleSource)
            .build());

    if (aligned) {
      middleSourceByteOffset = 0;
    } else {
      middleSourceByteOffset =
          (short)
              (firstLimbSingleSource
                  ? initialSourceByteOffset + firstLimbByteSize
                  : initialSourceByteOffset + firstLimbByteSize - LLARGE);
    }

    final Bytes dividend = longToBytes(middleSourceByteOffset + lastLimbByteSize - 1);
    final EucOperation eucOp = euc.callEUC(dividend, Bytes.of(LLARGE));
    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend.toLong())
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());

    lastLimbSingleSource =
        totNonTrivialIsOne ? firstLimbSingleSource : eucOp.quotient().toInt() == 0;
    initialSloIncrement = aligned ? true : !firstLimbSingleSource;
  }

  private void row5() {
    // row n°5
    final Bytes wcpArg1 = longToBytes(initialTargetByteOffset);
    final boolean wcpResult = wcp.callISZERO(wcpArg1);
    initialTboIsZero = wcpResult;
    wcpCallRecords.add(
        MmuWcpCallRecord.instIsZeroBuilder().arg1Lo(wcpArg1).result(wcpResult).build());

    final Bytes dividend = longToBytes(lastLimbByteSize);
    final EucOperation eucOp = euc.callEUC(dividend, Bytes.of(LLARGE));
    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend.toLong())
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());
    lastLimbIsFull = eucOp.quotient().toInt() == 1;
    lastLimbIsFast = aligned && lastLimbIsFull;
    if (!totNonTrivialIsOne) {
      firstLimbIsFast = aligned && initialTboIsZero;
    }
  }

  @Override
  public MmuData setMicroInstructions(final MmuData mmuData) {
    final HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    // Setting MMIO constant values
    mmuData.mmuToMmioConstantValues(
        MmuToMmioConstantValues.builder()
            .sourceContextNumber(hubToMmuValues.sourceId())
            .targetContextNumber(hubToMmuValues.targetId())
            .build());

    // Setting the list of MMIO instructions
    if (mmuData.totalNonTrivialInitials() == 1) {
      onlyMicroInstruction(mmuData);
    } else {
      firstMicroInstruction(mmuData);

      final int firstMiddleSlo =
          (int) (initialSloIncrement ? initialSourceLimbOffset + 1 : initialSourceLimbOffset);
      final int middleMicroInst =
          aligned ? MMIO_INST_RAM_TO_RAM_TRANSPLANT : MMIO_INST_RAM_TO_RAM_TWO_SOURCE;
      for (int i = 1; i < mmuData.totalNonTrivialInitials() - 1; i++) {
        middleMicroInstruction(mmuData, middleMicroInst, i, firstMiddleSlo);
      }
      lastMicroInstruction(mmuData, firstMiddleSlo);
    }

    return mmuData;
  }

  private void onlyMicroInstruction(final MmuData mmuData) {
    final int onlyMicroInst = calculateLastOrOnlyMicroInstruction();

    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(onlyMicroInst)
            .size(firstLimbByteSize)
            .sourceLimbOffset(initialSourceLimbOffset)
            .sourceByteOffset(initialSourceByteOffset)
            .targetLimbOffset(initialTargetLimbOffset)
            .targetByteOffset(initialTargetByteOffset)
            .build());
  }

  private void firstMicroInstruction(final MmuData mmuData) {
    final int onlyMicroInst = calculateFirstMicroInstruction();

    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(onlyMicroInst)
            .size(firstLimbByteSize)
            .sourceLimbOffset(initialSourceLimbOffset)
            .sourceByteOffset(initialSourceByteOffset)
            .targetLimbOffset(initialTargetLimbOffset)
            .targetByteOffset(initialTargetByteOffset)
            .build());
  }

  private void middleMicroInstruction(
      final MmuData mmuData,
      final int middleMicroInstruction,
      final int i,
      final int firstMiddleSlo) {

    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(middleMicroInstruction)
            .size((short) LLARGE)
            .sourceLimbOffset(firstMiddleSlo + i - 1)
            .sourceByteOffset(middleSourceByteOffset)
            .targetLimbOffset(initialTargetLimbOffset + i)
            .build());
  }

  private void lastMicroInstruction(final MmuData mmuData, final int firstMiddleSlo) {
    final int lastMicroInst = calculateLastOrOnlyMicroInstruction();

    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(lastMicroInst)
            .size(lastLimbByteSize)
            .sourceLimbOffset((firstMiddleSlo + totInitialNonTrivial - 2))
            .sourceByteOffset(middleSourceByteOffset)
            .targetLimbOffset((initialTargetLimbOffset + totInitialNonTrivial - 1))
            .build());
  }

  private int calculateLastOrOnlyMicroInstruction() {
    if (lastLimbIsFast) {
      return MMIO_INST_RAM_TO_RAM_TRANSPLANT;
    } else {
      return lastLimbSingleSource ? MMIO_INST_RAM_TO_RAM_PARTIAL : MMIO_INST_RAM_TO_RAM_TWO_SOURCE;
    }
  }

  private int calculateFirstMicroInstruction() {
    if (firstLimbIsFast) {
      return MMIO_INST_RAM_TO_RAM_TRANSPLANT;
    } else {
      return firstLimbSingleSource ? MMIO_INST_RAM_TO_RAM_PARTIAL : MMIO_INST_RAM_TO_RAM_TWO_SOURCE;
    }
  }
}
