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
import static net.consensys.linea.zktracer.Trace.LLARGEMO;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_ONE_TARGET;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_TWO_TARGET;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_EXCISION;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_PARTIAL;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_TWO_TARGET;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_VANISHES;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;
import static net.consensys.linea.zktracer.types.Conversions.longToBytes;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;

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
import org.apache.tuweni.bytes.Bytes;

public class AnyToRamWithPadding implements MmuInstruction {
  private final Euc euc;
  private final Wcp wcp;
  private List<MmuEucCallRecord> eucCallRecords;
  private List<MmuWcpCallRecord> wcpCallRecords;
  private long minTargetOffset;
  private long maxTargetOffset;
  private boolean purePadding;
  private long minTargetLimbOffset;
  private short minTargetByteOffset;
  private boolean mixed;
  private boolean pureData;
  private long maxTargetLimbOffset;
  private short maxTargetByteOffset;
  private long transferSize;
  private long paddingSize;
  private boolean lastPaddingIsFull;
  private short lastPaddingSize;
  private boolean totalRightZeroIsOne;
  private boolean firstPaddingIsFull;
  private boolean onlyPaddingIsFull;
  private short firstPaddingSize;
  private short onlyPaddingSize;
  private boolean targetLimbOffsetIncrementsAfterFirstDataTransfer;
  private boolean aligned;
  private short middleTargetByteOffset;
  private boolean lastDataTransferSingleTarget;
  private short lastDataTransferSize;
  private boolean targetLimbOffsetIncrementsAtTransition;
  private short firstPaddingByteOffset;
  private boolean dataSourceIsRam;
  private boolean totalNonTrivialIsOne;
  private short onlyDataTransferSize;
  private short firstDataTransferSize;
  private long minSourceOffset;
  private long minSourceLimbOffset;
  private short minSourceByteOffset;
  private long maxSourceLimbOffset;
  private short maxSourceByteOffset;
  private boolean onlyDataTransferSingleTarget;
  private boolean onlyDataTransferMaxesOutTarget;
  private boolean lastDataTransferMaxesOutTarget;
  private boolean firstDataTransferSingleTarget;
  private boolean firstDataTransferMaxesOutTarget;
  private long firstPaddingLimbOffset;
  private long lastPaddingLimbOffset;
  private int totInitialNonTrivial;
  private int totInitialRightZeroes;
  private long maxSourceOffsetOrZero;
  private long firstMiddleNonTrivialTargetLimbOffset;
  private boolean dataToPaddingTransitionTakesTwoMmioInstructions;

  public AnyToRamWithPadding(Euc euc, Wcp wcp) {
    this.euc = euc;
    this.wcp = wcp;
    this.eucCallRecords = new ArrayList<>();
    this.wcpCallRecords = new ArrayList<>();
  }

  @Override
  public MmuData preProcess(MmuData mmuData) {
    final HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    // Shared PreProcessing
    sharedCasePreProcessing(hubToMmuValues);
    // Setting subCase in MmuData
    mmuData.mmuInstAnyToRamWithPaddingIsPurePadding(purePadding);

    // PreProcessing depending on the subcase
    if (purePadding) {
      purePaddingCasePreProcessing();
    } else {
      someDataCasePreProcessing(hubToMmuValues);
    }

    mmuData.eucCallRecords(eucCallRecords);
    mmuData.wcpCallRecords(wcpCallRecords);

    // setting Out and Bin values
    if (purePadding) {
      purePaddingCaseOutAndBin(mmuData);
    } else {
      someDataCaseOutAndBin(mmuData);
    }

    // Setting Initial Number of MicroInstruction Type
    mmuData.totalLeftZeroesInitials(0);
    mmuData.totalNonTrivialInitials(totInitialNonTrivial);
    mmuData.totalRightZeroesInitials(totInitialRightZeroes);

    return mmuData;
  }

  private void someDataCasePreProcessing(final HubToMmuValues hubToMmuValues) {
    // The result of the computation of row 5 & 6 are used at row 3
    final MmuEucCallRecord eucOpRow5 = eucCallMinSourceOffset(hubToMmuValues);
    final MmuEucCallRecord eucOpRow6 = eucCallMaxSourceOffset();

    someDataRow3(hubToMmuValues);
    someDataRow4();
    someDataRow5(eucOpRow5);
    someDataRow6(eucOpRow6);
    someDataRow7();
    someDataRow8();
    someDataRow9();
    someDataRow10();
  }

  private void purePaddingCasePreProcessing() {
    purePaddingRow3();
    purePaddingRow4();
  }

  private void sharedCasePreProcessing(final HubToMmuValues hubToMmuValues) {
    sharedRow1(hubToMmuValues);
    sharedRow2(hubToMmuValues);
  }

  private void sharedRow1(final HubToMmuValues hubToMmuValues) {
    final boolean sourceOffsetHiIsZero = hubToMmuValues.sourceOffsetHi().equals(BigInteger.ZERO);
    final Bytes wcpArg1Hi =
        sourceOffsetHiIsZero ? Bytes.EMPTY : bigIntegerToBytes(hubToMmuValues.sourceOffsetHi());
    final Bytes wcpArg1Lo = bigIntegerToBytes(hubToMmuValues.sourceOffsetLo());
    final Bytes wcpArg1 =
        sourceOffsetHiIsZero
            ? wcpArg1Lo
            : Bytes.concatenate(wcpArg1Hi, leftPadTo(wcpArg1Lo, LLARGE));
    final Bytes wcpArg2 = longToBytes(hubToMmuValues.referenceSize());
    final boolean wcpResult = wcp.callLT(wcpArg1, wcpArg2);
    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder()
            .arg1Hi(wcpArg1Hi)
            .arg1Lo(wcpArg1Lo)
            .arg2Lo(wcpArg2)
            .result(wcpResult)
            .build());
    purePadding = !wcpResult;

    minTargetOffset = hubToMmuValues.targetOffset();
    maxTargetOffset = (minTargetOffset + hubToMmuValues.size() - 1);
    final long dividend = minTargetOffset;
    final EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));
    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend)
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());
    minTargetLimbOffset = eucOp.quotient().toLong();
    minTargetByteOffset = (short) eucOp.remainder().toInt();
    maxSourceOffsetOrZero =
        purePadding
            ? 0
            : (hubToMmuValues.sourceOffsetLo().longValueExact() + hubToMmuValues.size() - 1);
  }

  private void sharedRow2(final HubToMmuValues hubToMmuValues) {
    final Bytes wcpArg1 = longToBytes(maxSourceOffsetOrZero);
    final Bytes wcpArg2 = longToBytes(hubToMmuValues.referenceSize());
    final boolean wcpResult = wcp.callLT(wcpArg1, wcpArg2);
    wcpCallRecords.add(
        MmuWcpCallRecord.instLtBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(wcpResult).build());
    mixed = !purePadding && !wcpResult;
    pureData = !purePadding && wcpResult;

    final long dividend = maxTargetOffset;
    final EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));
    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend)
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());
    maxTargetLimbOffset = eucOp.quotient().toLong();
    maxTargetByteOffset = (short) eucOp.remainder().toInt();

    if (purePadding) {
      transferSize = 0;
      paddingSize = hubToMmuValues.size();
    }
    if (mixed) {
      transferSize =
          (hubToMmuValues.referenceSize() - hubToMmuValues.sourceOffsetLo().longValueExact());
      paddingSize = (hubToMmuValues.size() - transferSize);
    }
    if (pureData) {
      transferSize = hubToMmuValues.size();
      paddingSize = 0;
    }
  }

  private void purePaddingRow3() {
    totInitialNonTrivial = 0;
    totInitialRightZeroes = (int) (maxTargetLimbOffset - minTargetLimbOffset + 1);
    final Bytes wcpArg1 = longToBytes(totInitialRightZeroes);
    final Bytes wcpArg2 = Bytes.of(1);
    final boolean wcpResult = wcp.callEQ(wcpArg1, wcpArg2);
    wcpCallRecords.add(
        MmuWcpCallRecord.instEqBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(wcpResult).build());
    totalRightZeroIsOne = wcpResult;

    eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
  }

  private void purePaddingRow4() {
    final Bytes wcpArg1 = longToBytes(minTargetByteOffset);
    final boolean wcpResult = wcp.callISZERO(wcpArg1);
    wcpCallRecords.add(
        MmuWcpCallRecord.instIsZeroBuilder().arg1Lo(wcpArg1).result(wcpResult).build());

    final long dividend = maxTargetByteOffset + 1;
    final EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));
    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend)
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());

    firstPaddingIsFull = wcpResult;
    lastPaddingIsFull = !totalRightZeroIsOne && eucOp.quotient().toLong() == 1;
    onlyPaddingIsFull = firstPaddingIsFull && lastPaddingIsFull;
    firstPaddingSize = (short) (LLARGE - minTargetByteOffset);
    lastPaddingSize = (short) (totalRightZeroIsOne ? 0 : maxTargetByteOffset + 1);
    onlyPaddingSize = (short) paddingSize;
  }

  private MmuEucCallRecord eucCallMinSourceOffset(final HubToMmuValues hubToMmuValues) {
    minSourceOffset =
        (hubToMmuValues.sourceOffsetLo().longValueExact() + hubToMmuValues.referenceOffset());
    final long dividend = minSourceOffset;
    final EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));

    minSourceLimbOffset = eucOp.quotient().toLong();
    minSourceByteOffset = (short) eucOp.remainder().toInt();

    return MmuEucCallRecord.builder()
        .dividend(dividend)
        .divisor((short) LLARGE)
        .quotient(eucOp.quotient().toLong())
        .remainder((short) eucOp.remainder().toInt())
        .build();
  }

  private MmuEucCallRecord eucCallMaxSourceOffset() {
    final long dividend = minSourceOffset + transferSize - 1;
    final EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));
    maxSourceLimbOffset = eucOp.quotient().toLong();
    maxSourceByteOffset = (short) eucOp.remainder().toInt();
    return MmuEucCallRecord.builder()
        .dividend(dividend)
        .divisor((short) LLARGE)
        .quotient(eucOp.quotient().toLong())
        .remainder((short) eucOp.remainder().toInt())
        .build();
  }

  private void someDataRow3(final HubToMmuValues hubToMmuValues) {
    final Bytes wcpArg1 = longToBytes(hubToMmuValues.exoSum());
    final boolean wcpResult = wcp.callISZERO(wcpArg1);
    wcpCallRecords.add(
        MmuWcpCallRecord.instIsZeroBuilder().arg1Lo(wcpArg1).result(wcpResult).build());
    dataSourceIsRam = wcpResult;
    totInitialNonTrivial = (int) (maxSourceLimbOffset - minSourceLimbOffset + 1);

    eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
  }

  private void someDataRow4() {
    final Bytes wcpArg1 = longToBytes(totInitialNonTrivial);
    final Bytes wcpArg2 = Bytes.of(1);
    final boolean wcpResult = wcp.callEQ(wcpArg1, wcpArg2);
    wcpCallRecords.add(
        MmuWcpCallRecord.instEqBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(wcpResult).build());
    totalNonTrivialIsOne = wcpResult;
    onlyDataTransferSize = (short) transferSize;
    firstDataTransferSize = (short) (LLARGE - minSourceByteOffset);
    lastDataTransferSize = (short) (maxSourceByteOffset + 1);

    eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
  }

  private void someDataRow5(final MmuEucCallRecord eucOperation) {
    eucCallRecords.add(eucOperation);

    final Bytes wcpArg1 = longToBytes(minTargetByteOffset);
    final Bytes wcpArg2 = longToBytes(minSourceByteOffset);
    final boolean wcpResult = wcp.callEQ(wcpArg1, wcpArg2);
    wcpCallRecords.add(
        MmuWcpCallRecord.instEqBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(wcpResult).build());
    aligned = wcpResult;
  }

  private void someDataRow6(final MmuEucCallRecord eucOperation) {
    eucCallRecords.add(eucOperation);
    wcpCallRecords.add(MmuWcpCallRecord.EMPTY_CALL);
  }

  private void someDataRow7() {
    if (totalNonTrivialIsOne) {
      final long dividend = minTargetByteOffset + onlyDataTransferSize - 1;
      EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));
      eucCallRecords.add(
          MmuEucCallRecord.builder()
              .dividend(dividend)
              .divisor((short) LLARGE)
              .quotient(eucOp.quotient().toLong())
              .remainder((short) eucOp.remainder().toInt())
              .build());

      final Bytes wcpArg1 = eucOp.remainder();
      final Bytes wcpArg2 = Bytes.of(LLARGEMO);
      final boolean wcpResult = wcp.callEQ(wcpArg1, wcpArg2);
      wcpCallRecords.add(
          MmuWcpCallRecord.instEqBuilder()
              .arg1Lo(wcpArg1)
              .arg2Lo(wcpArg2)
              .result(wcpResult)
              .build());

      onlyDataTransferSingleTarget = eucOp.quotient().toLong() == 0;
      onlyDataTransferMaxesOutTarget = wcpResult;
    } else {
      eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
      wcpCallRecords.add(MmuWcpCallRecord.EMPTY_CALL);
    }
  }

  private void someDataRow8() {
    if (!totalNonTrivialIsOne) {
      final long dividend = minTargetByteOffset + firstDataTransferSize - 1;
      final EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));
      eucCallRecords.add(
          MmuEucCallRecord.builder()
              .dividend(dividend)
              .divisor((short) LLARGE)
              .quotient(eucOp.quotient().toLong())
              .remainder((short) eucOp.remainder().toInt())
              .build());

      final Bytes wcpArg1 = eucOp.remainder();
      final Bytes wcpArg2 = Bytes.of(LLARGEMO);
      final boolean wcpResult = wcp.callEQ(wcpArg1, wcpArg2);
      wcpCallRecords.add(
          MmuWcpCallRecord.instEqBuilder()
              .arg1Lo(wcpArg1)
              .arg2Lo(wcpArg2)
              .result(wcpResult)
              .build());

      firstDataTransferSingleTarget = eucOp.quotient().toLong() == 0;
      firstDataTransferMaxesOutTarget = wcpResult;
      middleTargetByteOffset =
          (short) (firstDataTransferMaxesOutTarget ? 0 : eucOp.remainder().toInt() + 1);
    } else {
      wcpCallRecords.add(MmuWcpCallRecord.EMPTY_CALL);
      eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
    }
  }

  private void someDataRow9() {
    if (!totalNonTrivialIsOne) {
      final long dividend = middleTargetByteOffset + lastDataTransferSize - 1;
      final EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));
      eucCallRecords.add(
          MmuEucCallRecord.builder()
              .dividend(dividend)
              .divisor((short) LLARGE)
              .quotient(eucOp.quotient().toLong())
              .remainder((short) eucOp.remainder().toInt())
              .build());

      final Bytes wcpArg1 = eucOp.remainder();
      final Bytes wcpArg2 = Bytes.of(LLARGEMO);
      final boolean wcpResult = wcp.callEQ(wcpArg1, wcpArg2);
      wcpCallRecords.add(
          MmuWcpCallRecord.instEqBuilder()
              .arg1Lo(wcpArg1)
              .arg2Lo(wcpArg2)
              .result(wcpResult)
              .build());
      lastDataTransferMaxesOutTarget = wcpResult;
      lastDataTransferSingleTarget = eucOp.quotient().toInt() == 0;
      targetLimbOffsetIncrementsAfterFirstDataTransfer =
          firstDataTransferSingleTarget ? firstDataTransferMaxesOutTarget : true;
      targetLimbOffsetIncrementsAtTransition =
          lastDataTransferSingleTarget ? lastDataTransferMaxesOutTarget : true;
    } else {
      wcpCallRecords.add(MmuWcpCallRecord.EMPTY_CALL);
      eucCallRecords.add(MmuEucCallRecord.EMPTY_CALL);
      targetLimbOffsetIncrementsAfterFirstDataTransfer = false;
      targetLimbOffsetIncrementsAtTransition =
          onlyDataTransferSingleTarget ? onlyDataTransferMaxesOutTarget : true;
    }
  }

  private void someDataRow10() {
    final long dividend = minTargetOffset + transferSize;
    final EucOperation eucOp = euc.callEUC(longToBytes(dividend), Bytes.of(LLARGE));
    eucCallRecords.add(
        MmuEucCallRecord.builder()
            .dividend(dividend)
            .divisor((short) LLARGE)
            .quotient(eucOp.quotient().toLong())
            .remainder((short) eucOp.remainder().toInt())
            .build());

    lastPaddingLimbOffset = maxTargetLimbOffset;
    firstPaddingLimbOffset = eucOp.quotient().toLong();
    firstPaddingByteOffset = (short) (mixed ? eucOp.remainder().toInt() : 0);
    totInitialRightZeroes =
        pureData ? 0 : (int) (lastPaddingLimbOffset - firstPaddingLimbOffset + 1);

    final Bytes wcpArg1 = longToBytes(totInitialRightZeroes);
    final Bytes wcpArg2 = Bytes.of(1);
    final boolean wcpResult = wcp.callEQ(wcpArg1, wcpArg2);
    wcpCallRecords.add(
        MmuWcpCallRecord.instEqBuilder().arg1Lo(wcpArg1).arg2Lo(wcpArg2).result(wcpResult).build());

    totalRightZeroIsOne = wcpResult;
    final int lastPaddingByteOffset = maxTargetByteOffset;

    if (totalRightZeroIsOne) {
      firstPaddingSize = (short) paddingSize;
      lastPaddingSize = 0;
    } else {
      firstPaddingSize = (short) (mixed ? LLARGE - firstPaddingByteOffset : 0);
      lastPaddingSize = (short) (mixed ? lastPaddingByteOffset + 1 : 0);
    }
  }

  private void someDataCaseOutAndBin(MmuData mmuData) {
    mmuData.outAndBinValues(
        MmuOutAndBinValues.builder()
            .bin1(targetLimbOffsetIncrementsAfterFirstDataTransfer)
            .bin2(aligned)
            .bin3(lastDataTransferSingleTarget)
            .bin4(targetLimbOffsetIncrementsAtTransition)
            .bin5(dataSourceIsRam)
            .out1(middleTargetByteOffset)
            .out2(lastDataTransferSize)
            .out3(firstPaddingByteOffset)
            .out4(firstPaddingSize)
            .out5(lastPaddingSize)
            .build());
  }

  private void purePaddingCaseOutAndBin(MmuData mmuData) {
    mmuData.outAndBinValues(
        MmuOutAndBinValues.builder().bin1(lastPaddingIsFull).out1(lastPaddingSize).build());
  }

  @Override
  public MmuData setMicroInstructions(MmuData mmuData) {
    if (purePadding) {
      setMicroInstructionsPurePaddingCase(mmuData);
    } else {
      setMicroInstructionsSomeDataCase(mmuData);
    }

    return mmuData;
  }

  private void setMicroInstructionsPurePaddingCase(MmuData mmuData) {
    final HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    mmuData.mmuToMmioConstantValues(
        MmuToMmioConstantValues.builder().targetContextNumber(hubToMmuValues.targetId()).build());

    if (totalRightZeroIsOne) {
      purePaddingOnlyMicroInstruction(mmuData);
    } else {
      purePaddingFirstMicroInstruction(mmuData);
      for (int i = 1; i < totInitialRightZeroes - 1; i++) {
        purePaddingMiddleMicroInstruction(mmuData, i);
      }
      purePaddingLastMicroInstruction(mmuData);
    }
  }

  private void setMicroInstructionsSomeDataCase(MmuData mmuData) {
    final HubToMmuValues hubToMmuValues = mmuData.hubToMmuValues();

    // Setting if the transition data / padding is made in 1 or 2 mmio instructions
    dataToPaddingTransitionTakesTwoMmioInstructions =
        totInitialRightZeroes != 0
            && (!onlyDataTransferMaxesOutTarget || !lastDataTransferMaxesOutTarget);

    // Setting Microinstruction constant values
    mmuData.mmuToMmioConstantValues(
        MmuToMmioConstantValues.builder()
            .sourceContextNumber(dataSourceIsRam ? hubToMmuValues.sourceId() : 0)
            .targetContextNumber(hubToMmuValues.targetId())
            .exoSum(hubToMmuValues.exoSum())
            .exoId(dataSourceIsRam ? 0 : (int) hubToMmuValues.sourceId())
            .totalSize((int) hubToMmuValues.referenceSize())
            .build());

    // Setting data transfer micro instructions
    if (totalNonTrivialIsOne) {
      someDataOnlyNonTrivialInstruction(mmuData);
    } else {
      someDataFirstNonTrivialInstruction(mmuData);
      firstMiddleNonTrivialTargetLimbOffset =
          targetLimbOffsetIncrementsAfterFirstDataTransfer
              ? minTargetLimbOffset + 1
              : minTargetLimbOffset;
      for (int i = 1; i < totInitialNonTrivial - 1; i++) {
        someDataMiddleNonTrivialInstruction(mmuData, i);
      }
      someDataLastNonTrivialInstruction(mmuData);
    }

    // Setting padding micro instructions
    if (totInitialRightZeroes != 0) {
      someDataOnlyOrFirstPaddingInstruction(mmuData);
      if (!totalRightZeroIsOne) {
        for (int i = 1; i < totInitialRightZeroes - 1; i++) {
          someDataMiddlePaddingInstruction(mmuData, i);
        }
        someDataLastPaddingInstruction(mmuData);
      }
    }
  }

  private void purePaddingOnlyMicroInstruction(MmuData mmuData) {
    final int onlyMicroInst = onlyPaddingIsFull ? MMIO_INST_RAM_VANISHES : MMIO_INST_RAM_EXCISION;
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(onlyMicroInst)
            .size(onlyPaddingSize)
            .targetLimbOffset(minTargetLimbOffset)
            .targetByteOffset(minTargetByteOffset)
            .build());
  }

  private void purePaddingFirstMicroInstruction(MmuData mmuData) {
    final int firstMicroInst = firstPaddingIsFull ? MMIO_INST_RAM_VANISHES : MMIO_INST_RAM_EXCISION;
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(firstMicroInst)
            .size(firstPaddingSize)
            .targetLimbOffset(minTargetLimbOffset)
            .targetByteOffset(minTargetByteOffset)
            .build());
  }

  private void purePaddingMiddleMicroInstruction(MmuData mmuData, int rowNumber) {
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(MMIO_INST_RAM_VANISHES)
            .targetLimbOffset(minTargetLimbOffset + rowNumber)
            .build());
  }

  private void purePaddingLastMicroInstruction(MmuData mmuData) {
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(lastPaddingIsFull ? MMIO_INST_RAM_VANISHES : MMIO_INST_RAM_EXCISION)
            .size(lastPaddingSize)
            .targetLimbOffset(maxTargetLimbOffset)
            .build());
  }

  private void someDataOnlyNonTrivialInstruction(MmuData mmuData) {
    int onlyMmioInstruction;
    if (dataSourceIsRam) {
      onlyMmioInstruction =
          onlyDataTransferSingleTarget
              ? MMIO_INST_RAM_TO_RAM_PARTIAL
              : MMIO_INST_RAM_TO_RAM_TWO_TARGET;
    } else {
      onlyMmioInstruction =
          onlyDataTransferSingleTarget
              ? MMIO_INST_LIMB_TO_RAM_ONE_TARGET
              : MMIO_INST_LIMB_TO_RAM_TWO_TARGET;
    }

    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(onlyMmioInstruction)
            .size(onlyDataTransferSize)
            .sourceLimbOffset(minSourceLimbOffset)
            .sourceByteOffset(minSourceByteOffset)
            .targetLimbOffset(minTargetLimbOffset)
            .targetByteOffset(minTargetByteOffset)
            //  .limb(limb)
            .targetLimbIsTouchedTwice(
                mmioInstTouchesTwoRamTarget(onlyMmioInstruction)
                    || dataToPaddingTransitionTakesTwoMmioInstructions)
            .build());
  }

  private void someDataFirstNonTrivialInstruction(MmuData mmuData) {
    int firstMmioInstruction;
    if (dataSourceIsRam) {
      firstMmioInstruction =
          firstDataTransferSingleTarget
              ? MMIO_INST_RAM_TO_RAM_PARTIAL
              : MMIO_INST_RAM_TO_RAM_TWO_TARGET;
    } else {
      firstMmioInstruction =
          firstDataTransferSingleTarget
              ? MMIO_INST_LIMB_TO_RAM_ONE_TARGET
              : MMIO_INST_LIMB_TO_RAM_TWO_TARGET;
    }

    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(firstMmioInstruction)
            .size(firstDataTransferSize)
            .sourceLimbOffset(minSourceLimbOffset)
            .sourceByteOffset(minSourceByteOffset)
            .targetLimbOffset(minTargetLimbOffset)
            .targetByteOffset(minTargetByteOffset)
            .targetLimbIsTouchedTwice(
                !targetLimbOffsetIncrementsAfterFirstDataTransfer
                    || mmioInstTouchesTwoRamTarget(firstMmioInstruction))
            .build());
  }

  private void someDataMiddleNonTrivialInstruction(MmuData mmuData, int rowNumber) {
    final long sourceLimbOffset = minSourceLimbOffset + rowNumber;
    int middleMmioInstruction;
    if (dataSourceIsRam) {
      middleMmioInstruction =
          aligned ? MMIO_INST_RAM_TO_RAM_TRANSPLANT : MMIO_INST_RAM_TO_RAM_TWO_TARGET;
    } else {
      middleMmioInstruction =
          aligned ? MMIO_INST_LIMB_TO_RAM_TRANSPLANT : MMIO_INST_LIMB_TO_RAM_TWO_TARGET;
    }

    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(middleMmioInstruction)
            .size((short) LLARGE)
            .sourceLimbOffset(sourceLimbOffset)
            .targetLimbOffset(firstMiddleNonTrivialTargetLimbOffset + rowNumber - 1)
            .targetByteOffset(middleTargetByteOffset)
            .targetLimbIsTouchedTwice(mmioInstTouchesTwoRamTarget(middleMmioInstruction))
            .build());
  }

  private void someDataLastNonTrivialInstruction(MmuData mmuData) {
    final long sourceLimbOffset = minSourceLimbOffset + totInitialNonTrivial - 1;
    int lastMmioInstruction;
    if (dataSourceIsRam) {
      lastMmioInstruction =
          lastDataTransferSingleTarget
              ? MMIO_INST_RAM_TO_RAM_PARTIAL
              : MMIO_INST_RAM_TO_RAM_TWO_TARGET;
    } else {
      lastMmioInstruction =
          lastDataTransferSingleTarget
              ? MMIO_INST_LIMB_TO_RAM_ONE_TARGET
              : MMIO_INST_LIMB_TO_RAM_TWO_TARGET;
    }
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(lastMmioInstruction)
            .size(lastDataTransferSize)
            .sourceLimbOffset(sourceLimbOffset)
            .targetLimbOffset(firstMiddleNonTrivialTargetLimbOffset + totInitialNonTrivial - 2)
            .targetByteOffset(middleTargetByteOffset)
            .targetLimbIsTouchedTwice(
                mmioInstTouchesTwoRamTarget(lastMmioInstruction)
                    || dataToPaddingTransitionTakesTwoMmioInstructions)
            .build());
  }

  private void someDataOnlyOrFirstPaddingInstruction(MmuData mmuData) {
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(MMIO_INST_RAM_EXCISION)
            .size(firstPaddingSize)
            .targetLimbOffset(firstPaddingLimbOffset)
            .targetByteOffset(firstPaddingByteOffset)
            .build());
  }

  private void someDataMiddlePaddingInstruction(MmuData mmuData, int rowNumber) {
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(MMIO_INST_RAM_VANISHES)
            .targetLimbOffset(firstPaddingLimbOffset + rowNumber)
            .build());
  }

  private void someDataLastPaddingInstruction(MmuData mmuData) {
    mmuData.mmuToMmioInstruction(
        MmuToMmioInstruction.builder()
            .mmioInstruction(MMIO_INST_RAM_EXCISION)
            .size(lastPaddingSize)
            .targetLimbOffset(lastPaddingLimbOffset)
            .build());
  }

  private boolean mmioInstTouchesTwoRamTarget(int mmioInstruction) {
    return mmioInstruction == MMIO_INST_RAM_TO_RAM_TWO_TARGET
        || mmioInstruction == MMIO_INST_LIMB_TO_RAM_TWO_TARGET;
  }
}
