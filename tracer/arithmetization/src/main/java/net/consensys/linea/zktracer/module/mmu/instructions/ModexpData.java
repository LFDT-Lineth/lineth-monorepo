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

import static net.consensys.linea.zktracer.Trace.Blake2fmodexpdata.MODEXP_MAX_BYTE_SIZE;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_VANISHES;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_ONE_SOURCE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
import static net.consensys.linea.zktracer.Trace.Mmu.*;
import static net.consensys.linea.zktracer.TraceOsaka.Blake2fmodexpdata.INDEX_MAX_MODEXP;
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

public class ModexpData implements MmuInstruction {
  private final Euc euc;
  private final Wcp wcp;
  private final List<MmuEucCallRecord> eucCallRecords;
  private final List<MmuWcpCallRecord> wcpCallRecords;
  private int initialTotalLeftZeroes;
  private int initialTotalNonTrivial;
  private int initialTotalRightZeroes;
  private short initialTargetByteOffset;
  private long initialSourceLimbOffset;
  private short initialSourceByteOffset;
  private short firstLimbByteSize;
  private short lastLimbByteSize;
  private boolean firstLimbSingleSource;
  private boolean aligned;
  private boolean lastLimbSingleSource;
  private int parameterByteSize;
  private int parameterOffset;
  private int leftoverDataSize;
  private boolean dataRunsOut;
  private int rightPaddingRemainder;
  private short middleSourceByteOffset;
  private long middleFirstSourceLimbOffset;
  private int middleMicroInst;

  public ModexpData(Euc euc, Wcp wcp) {
    this.euc = euc;
    this.wcp = wcp;
    this.eucCallRecords = new ArrayList<>(NB_PP_ROWS_MODEXP_DATA);
    this.wcpCallRecords = new ArrayList<>(NB_PP_ROWS_MODEXP_DATA);
  }

  private int getForkAppropriateModexpInputSize() {
    return LLARGE * (INDEX_MAX_MODEXP + 1);
  }

  @Override
  public MmuData preProcess(MmuData mmuData) {
    final HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();
    row1(hubToMmuValues);
    row2();
    row3();
    row4();
    row5();
    row6();

    mmuData.eucCallRecords(eucCallRecords);
    mmuData.wcpCallRecords(wcpCallRecords);

    // setting Out and Bin values
    mmuData.outAndBinValues(
        MmuOutAndBinValues.builder()
            .out1(initialTargetByteOffset)
            .out2(initialSourceLimbOffset)
            .out3(initialSourceByteOffset)
            .out4(firstLimbByteSize)
            .out5(lastLimbByteSize)
            .bin1(firstLimbSingleSource)
            .bin2(aligned)
            .bin3(lastLimbSingleSource)
            .build());

    mmuData.totalLeftZeroesInitials(initialTotalLeftZeroes);
    mmuData.totalNonTrivialInitials(initialTotalNonTrivial);
    mmuData.totalRightZeroesInitials(initialTotalRightZeroes);

    return mmuData;
  }

  private void row1(final HubToMmuValues hubToMmuValues) {
    // row n°1
    parameterByteSize = (int) hubToMmuValues.size();
    parameterOffset =
        (int) (hubToMmuValues.referenceOffset() + hubToMmuValues.sourceOffsetLo().longValueExact());
    leftoverDataSize =
        (int) (hubToMmuValues.referenceSize() - hubToMmuValues.sourceOffsetLo().longValueExact());
    final int numberLeftPaddingBytes = MODEXP_MAX_BYTE_SIZE - parameterByteSize;

    final EucOperation eucOp = euc.callEUC(longToBytes(numberLeftPaddingBytes), Bytes.of(LLARGE));

    initialTargetByteOffset = (short) eucOp.remainder().toInt();
    initialTotalLeftZeroes = eucOp.quotient().toInt();

    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(numberLeftPaddingBytes)
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());

    wcpCallRecords.add(MmuWcpCallRecord.EMPTY_CALL);
  }

  private void row2() {
    // row n°2
    final Bytes wcpArg1 = longToBytes(leftoverDataSize);
    final Bytes wcpArg2 = longToBytes(parameterByteSize);
    final boolean wcpResult = wcp.callLT(wcpArg1, wcpArg2);
    dataRunsOut = wcpResult;
    final int numberRightPaddingBytes = dataRunsOut ? parameterByteSize - leftoverDataSize : 0;
    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(wcpResult).build());

    final EucOperation eucOp = euc.callEUC(longToBytes(numberRightPaddingBytes), Bytes.of(LLARGE));
    initialTotalRightZeroes = eucOp.quotient().toInt();
    initialTotalNonTrivial =
        NB_MICRO_ROWS_TOT_MODEXP_ZERO - initialTotalLeftZeroes - initialTotalRightZeroes;
    rightPaddingRemainder = eucOp.remainder().toInt();
    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(numberRightPaddingBytes)
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());
  }

  private void row3() {
    // row n°3
    final Bytes wcpArg1 = longToBytes(initialTotalNonTrivial);
    final Bytes wcpArg2 = Bytes.of(1);
    final boolean singleNonTrivialMmioOperation = wcp.callEQ(wcpArg1, wcpArg2);
    wcpCallRecords.add(
        MmuWcpCallRecord.instEqBuilder()
            .arg1Lo(wcpArg1)
            .arg2Lo(wcpArg2)
            .result(singleNonTrivialMmioOperation)
            .build());

    final long dividend = parameterOffset;
    final EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));
    initialSourceLimbOffset = eucOp.quotient().toLong();
    initialSourceByteOffset = (short) eucOp.remainder().toInt();
    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend)
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());
    if (singleNonTrivialMmioOperation) {
      firstLimbByteSize = (short) (dataRunsOut ? leftoverDataSize : parameterByteSize);
    } else {
      firstLimbByteSize = (short) (LLARGE - initialTargetByteOffset);
    }
    lastLimbByteSize = (short) (dataRunsOut ? LLARGE - rightPaddingRemainder : LLARGE);
  }

  private void row4() {
    // row n°4
    final Bytes wcpArg1 = longToBytes(initialSourceByteOffset + firstLimbByteSize - 1);
    final Bytes wcpArg2 = Bytes.of(LLARGE);
    final boolean wcpResult = wcp.callLT(wcpArg1, wcpArg2);
    firstLimbSingleSource = wcpResult;
    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(wcpResult).build());

    eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
  }

  private void row5() {
    // row n°5
    final Bytes wcpArg1 = longToBytes(initialSourceByteOffset);
    final Bytes wcpArg2 = Bytes.of(initialTargetByteOffset);
    final boolean wcpResult = wcp.callEQ(wcpArg1, wcpArg2);
    aligned = wcpResult;
    wcpCallRecords.add(
        MmuWcpCallRecord.instEqBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(wcpResult).build());

    eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
  }

  private void row6() {
    // row n°6
    if (aligned) {
      lastLimbSingleSource = true;
      eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
      wcpCallRecords.add(MmuWcpCallRecord.EMPTY_CALL);
    } else {
      final long dividend = initialSourceByteOffset + firstLimbByteSize;
      final EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));
      middleSourceByteOffset = (short) eucOp.remainder().toInt();
      eucCallRecords.add(
          MmuEucCallRecord.builder()
              .dividend(dividend)
              .divisor((short) LLARGE)
              .quotient(eucOp.quotient().toLong())
              .remainder((short) eucOp.remainder().toInt())
              .build());

      final Bytes wcpArg1 = longToBytes(middleSourceByteOffset + lastLimbByteSize - 1);
      final Bytes wcpArg2 = Bytes.of(LLARGE);
      final boolean wcpResult = wcp.callLT(wcpArg1, wcpArg2);
      lastLimbSingleSource = wcpResult;
      wcpCallRecords.add(
          MmuWcpCallRecord.instLtBuilder()
              .arg1Lo(wcpArg1)
              .arg2Lo(wcpArg2)
              .result(wcpResult)
              .build());
    }
  }

  @Override
  public MmuData setMicroInstructions(MmuData mmuData) {
    HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    // Setting MMIO constant values
    mmuData.mmuToMmioConstantValues(
        MmuToMmioConstantValues.builder()
            .sourceContextNumber(hubToMmuValues.sourceId())
            .exoSum(hubToMmuValues.exoSum())
            .phase(hubToMmuValues.phase())
            .exoId((int) hubToMmuValues.targetId())
            .build());

    // Left Zeroes
    for (int i = 0; i < initialTotalLeftZeroes; i++) {
      vanishingMicroInstruction(mmuData, i);
    }

    // Non-Trivial Rows
    firstOrOnlyMicroInstruction(mmuData);

    middleFirstSourceLimbOffset = determineFirstMiddleSourceLimbOffset();
    middleMicroInst = aligned ? MMIO_INST_RAM_TO_LIMB_TRANSPLANT : MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
    for (int i = 1; i < mmuData.totalNonTrivialInitials() - 1; i++) {
      final long sourceLimbOffset = middleFirstSourceLimbOffset + i - 1;
      final int targetLimbOffset = initialTotalLeftZeroes + i;
      middleMicroInstruction(mmuData, sourceLimbOffset, targetLimbOffset);
    }

    if (initialTotalNonTrivial > 1) {
      lastMicroInstruction(mmuData);
    }

    // Right Zeroes
    for (int i = 0; i < initialTotalRightZeroes; i++) {
      vanishingMicroInstruction(mmuData, initialTotalLeftZeroes + initialTotalNonTrivial + i);
    }

    return mmuData;
  }

  private void vanishingMicroInstruction(MmuData mmuData, final int i) {
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(MMIO_INST_LIMB_VANISHES)
            .targetLimbOffset(i)
            .build());
  }

  private void firstOrOnlyMicroInstruction(MmuData mmuData) {
    final int firstMicroInst =
        firstLimbSingleSource ? MMIO_INST_RAM_TO_LIMB_ONE_SOURCE : MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(firstMicroInst)
            .size(firstLimbByteSize)
            .sourceLimbOffset(initialSourceLimbOffset)
            .sourceByteOffset(initialSourceByteOffset)
            .targetLimbOffset(initialTotalLeftZeroes)
            .targetByteOffset(initialTargetByteOffset)
            .build());
  }

  private void middleMicroInstruction(
      MmuData mmuData, final long sourceLimbOffset, final int targetLimbOffset) {
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(middleMicroInst)
            .size((short) LLARGE)
            .sourceLimbOffset(sourceLimbOffset)
            .sourceByteOffset(middleSourceByteOffset)
            .targetLimbOffset(targetLimbOffset)
            .build());
  }

  private void lastMicroInstruction(MmuData mmuData) {
    final int lastMicroInstruction =
        lastLimbSingleSource ? MMIO_INST_RAM_TO_LIMB_ONE_SOURCE : MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
    final long sourceLimbOffset = middleFirstSourceLimbOffset + initialTotalNonTrivial - 2;

    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(lastMicroInstruction)
            .size(lastLimbByteSize)
            .sourceLimbOffset(sourceLimbOffset)
            .sourceByteOffset(middleSourceByteOffset)
            .targetLimbOffset(initialTotalLeftZeroes + initialTotalNonTrivial - 1)
            .build());
  }

  private long determineFirstMiddleSourceLimbOffset() {
    if (aligned) return initialSourceLimbOffset + 1;
    return firstLimbSingleSource ? initialSourceLimbOffset : initialSourceLimbOffset + 1;
  }
}
