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

package net.consensys.linea.zktracer.module.mmio;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.LLARGEMO;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_ONE_TARGET;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_TO_RAM_TWO_TARGET;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_LIMB_VANISHES;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_ONE_SOURCE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_LIMB_TWO_SOURCE;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_TO_RAM_TRANSPLANT;
import static net.consensys.linea.zktracer.Trace.MMIO_INST_RAM_VANISHES;
import static net.consensys.linea.zktracer.module.mmio.MmioPatterns.antiPower;
import static net.consensys.linea.zktracer.module.mmio.MmioPatterns.isolateChunk;
import static net.consensys.linea.zktracer.module.mmio.MmioPatterns.isolatePrefix;
import static net.consensys.linea.zktracer.module.mmio.MmioPatterns.isolateSuffix;
import static net.consensys.linea.zktracer.module.mmio.MmioPatterns.plateau;
import static net.consensys.linea.zktracer.module.mmio.MmioPatterns.power;

import java.util.ArrayList;
import java.util.List;

import lombok.AllArgsConstructor;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.mmu.values.HubToMmuValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuToMmioConstantValues;
import net.consensys.linea.zktracer.module.mmu.values.MmuToMmioInstruction;
import net.consensys.linea.zktracer.types.Bytes16;
import org.apache.tuweni.bytes.Bytes;

@Getter
@Setter
@Accessors(fluent = true)
@AllArgsConstructor
public class MmioData {
  private long cnA;
  private long cnB;
  private long cnC;

  private long indexA;
  private long indexB;
  private long indexC;

  private Bytes16 valA;
  private Bytes16 valB;
  private Bytes16 valC;

  private Bytes16 valANew;
  private Bytes16 valBNew;
  private Bytes16 valCNew;

  // imported from the mmu
  private final int instruction;
  private final long sourceContext;
  private final long targetContext;
  private final long sourceLimbOffset;
  private final long targetLimbOffset;
  private final short sourceByteOffset;
  private final short targetByteOffset;
  private final short size;
  private Bytes16 limb;
  private final long totalSize;
  private final int exoSum;
  private final boolean exoIsRom;
  private final boolean exoIsBlake2fModexp;
  private final boolean exoIsEcData;
  private final boolean exoIsBls;
  private final boolean exoIsRipSha;
  private final boolean exoIsKeccak;
  private final boolean exoIsLog;
  private final boolean exoIsTxcd;
  private final int exoId;
  private final int kecId;
  private final int phase;
  private final boolean successBit;
  private final boolean targetLimbIsTouchedTwice;

  private long indexX;

  private List<Boolean> bit1;
  private List<Boolean> bit2;
  private List<Boolean> bit3;
  private List<Boolean> bit4;
  private List<Boolean> bit5;

  private List<Bytes> pow2561;
  private List<Bytes> pow2562;

  private List<Bytes> acc1;
  private List<Bytes> acc2;
  private List<Bytes> acc3;
  private List<Bytes> acc4;

  public MmioData(
      HubToMmuValues hubToMmuValues,
      MmuToMmioConstantValues mmuToMmioConstantValues,
      MmuToMmioInstruction mmuToMmioInstruction) {
    this(
        0,
        0,
        0,
        0,
        0,
        0,
        Bytes16.ZERO,
        Bytes16.ZERO,
        Bytes16.ZERO,
        Bytes16.ZERO,
        Bytes16.ZERO,
        Bytes16.ZERO,
        mmuToMmioInstruction.mmioInstruction(),
        mmuToMmioConstantValues.sourceContextNumber(),
        mmuToMmioConstantValues.targetContextNumber(),
        mmuToMmioInstruction.sourceLimbOffset(),
        mmuToMmioInstruction.targetLimbOffset(),
        mmuToMmioInstruction.sourceByteOffset(),
        mmuToMmioInstruction.targetByteOffset(),
        mmuToMmioInstruction.size(),
        mmuToMmioInstruction.limb(),
        mmuToMmioConstantValues.totalSize(),
        mmuToMmioConstantValues.exoSum(),
        hubToMmuValues.exoIsRom(),
        hubToMmuValues.exoIsBlake2fModexp(),
        hubToMmuValues.exoIsEcData(),
        hubToMmuValues.exoIsBls(),
        hubToMmuValues.exoIsRipSha(),
        hubToMmuValues.exoIsKeccak(),
        hubToMmuValues.exoIsLog(),
        hubToMmuValues.exoIsTxcd(),
        mmuToMmioConstantValues.exoId(),
        mmuToMmioConstantValues.kecId(),
        mmuToMmioConstantValues.phase(),
        mmuToMmioConstantValues.successBit(),
        mmuToMmioInstruction.targetLimbIsTouchedTwice(),
        0,
        new ArrayList<>(LLARGE),
        new ArrayList<>(LLARGE),
        new ArrayList<>(LLARGE),
        new ArrayList<>(LLARGE),
        new ArrayList<>(LLARGE),
        new ArrayList<>(LLARGE),
        new ArrayList<>(LLARGE),
        new ArrayList<>(LLARGE),
        new ArrayList<>(LLARGE),
        new ArrayList<>(LLARGE),
        new ArrayList<>(LLARGE));
  }

  public static boolean isFastOperation(final int mmioInstruction) {
    return (mmioInstruction == MMIO_INST_LIMB_VANISHES
        || mmioInstruction == MMIO_INST_LIMB_TO_RAM_TRANSPLANT
        || mmioInstruction == MMIO_INST_RAM_TO_LIMB_TRANSPLANT
        || mmioInstruction == MMIO_INST_RAM_TO_RAM_TRANSPLANT
        || mmioInstruction == MMIO_INST_RAM_VANISHES);
  }

  public static int lineCountOfMmioInstruction(final int mmioInstruction) {
    return isFastOperation(mmioInstruction) ? 1 : LLARGE;
  }

  public void onePartialToOne(
      final Bytes16 sourceBytes,
      final Bytes16 targetBytes,
      final short sourceOffsetTrigger,
      final short targetOffsetTrigger,
      final short size) {
    for (short ct = 0; ct < LLARGE; ct++) {
      bit1.add(ct, plateau(targetOffsetTrigger, ct));
      bit2.add(ct, plateau(targetOffsetTrigger + size, ct));
      bit3.add(ct, plateau(sourceOffsetTrigger, ct));
      bit4.add(ct, plateau(sourceOffsetTrigger + size, ct));
    }
    acc1 = isolateChunk(targetBytes, bit1, bit2);
    acc2 = isolateChunk(sourceBytes, bit3, bit4);
    pow2561 = power(bit2);
  }

  public void onePartialToTwo(
      final Bytes16 sourceBytes,
      final Bytes16 target1Bytes,
      final Bytes16 target2Bytes,
      final short sourceOffsetTrigger,
      final short target1OffsetTrigger,
      final short size) {
    for (short ct = 0; ct < LLARGE; ct++) {
      bit1.add(ct, plateau(target1OffsetTrigger, ct));
      bit2.add(ct, plateau(target1OffsetTrigger + size - LLARGE, ct));
      bit3.add(ct, plateau(sourceOffsetTrigger, ct));
      bit4.add(ct, plateau(sourceOffsetTrigger + LLARGE - target1OffsetTrigger, ct));
      bit5.add(ct, plateau(sourceOffsetTrigger + size, ct));
    }
    acc1 = isolateSuffix(target1Bytes, bit1);
    acc2 = isolatePrefix(target2Bytes, bit2);
    acc3 = isolateChunk(sourceBytes, bit3, bit4);
    acc4 = isolateChunk(sourceBytes, bit4, bit5);
    pow2561 = power(bit2);
  }

  public void oneToOnePadded(
      final Bytes16 sourceBytes,
      final short sourceByteOffset,
      final short targetByteOffset,
      final short size) {

    checkArgument(
        0 <= sourceByteOffset && sourceByteOffset <= LLARGEMO,
        "sourceByteOffset has value %s.",
        sourceByteOffset);
    checkArgument(0 < size && size <= LLARGE, "size has value %s.", size);
    checkArgument(
        sourceByteOffset + size - 1 <= LLARGEMO,
        "sourceByteOffset has value %s.",
        sourceByteOffset);
    checkArgument(
        0 <= targetByteOffset && targetByteOffset <= LLARGEMO,
        "targetByteOffset has value %s.",
        targetByteOffset);
    checkArgument(
        targetByteOffset + size - 1 <= LLARGEMO,
        "targetByteOffset has value %s.",
        targetByteOffset);

    for (short ct = 0; ct < LLARGE; ct++) {
      bit1.add(ct, plateau(sourceByteOffset, ct));
      bit2.add(ct, plateau(sourceByteOffset + size, ct));
      bit3.add(ct, plateau(targetByteOffset + size, ct));
    }
    acc1 = isolateChunk(sourceBytes, bit1, bit2);
    pow2561 = power(bit3);
  }

  public void excision(final Bytes16 target, final short targetOffsetTrigger, final short size) {

    for (short ct = 0; ct < LLARGE; ct++) {
      bit1.add(ct, plateau(targetOffsetTrigger, ct));
      bit2.add(ct, plateau(targetOffsetTrigger + size, ct));
    }

    acc1 = isolateChunk(target, bit1, bit2);
    pow2561 = power(bit2);
  }

  public void twoToOnePadded(
      final Bytes16 sourceBytes1,
      final Bytes16 sourceBytes2,
      final short sourceByteOffset,
      final short targetByteOffset,
      final short size) {

    checkArgument(
        0 <= sourceByteOffset && sourceByteOffset <= LLARGEMO,
        "sourceByteOffset has value %s.",
        sourceByteOffset);
    checkArgument(0 < size && size <= LLARGE, "size has value %s.", size);
    checkArgument(
        sourceByteOffset + size - 1 > LLARGEMO, "sourceByteOffset has value %s.", sourceByteOffset);
    checkArgument(
        0 <= targetByteOffset && targetByteOffset <= LLARGEMO,
        "targetByteOffset has value %s.",
        targetByteOffset);
    checkArgument(
        targetByteOffset + size - 1 <= LLARGEMO,
        "targetByteOffset has value %s.",
        targetByteOffset);

    for (short ct = 0; ct < LLARGE; ct++) {
      bit1.add(ct, plateau(sourceByteOffset, ct));
      bit2.add(ct, plateau(sourceByteOffset + size - LLARGE, ct));
      bit3.add(ct, plateau(targetByteOffset + LLARGE - sourceByteOffset, ct));
      bit4.add(ct, plateau(targetByteOffset + size, ct));
    }
    acc1 = isolateSuffix(sourceBytes1, bit1);
    acc2 = isolatePrefix(sourceBytes2, bit2);
    pow2561 = power(bit3);
    pow2562 = power(bit4);
  }

  public void twoPartialToOne(
      final Bytes16 source1,
      final Bytes16 source2,
      final Bytes16 target,
      final short sourceByteOffset,
      final short targetByteOffset,
      final short size) {

    for (short ct = 0; ct < LLARGE; ct++) {
      bit1.add(ct, plateau(sourceByteOffset, ct));
      bit2.add(ct, plateau(sourceByteOffset + size - LLARGE, ct));
      bit3.add(ct, plateau(targetByteOffset, ct));
      bit4.add(ct, plateau(targetByteOffset + size, ct));
    }

    acc1 = isolateSuffix(source1, bit1);
    acc2 = isolatePrefix(source2, bit2);
    acc3 = isolateChunk(target, bit3, bit4);

    pow2561 = power(bit4);
    pow2562 = antiPower(bit2);
  }

  public boolean operationRequiresExoFlag() {
    return List.of(
            MMIO_INST_LIMB_VANISHES,
            MMIO_INST_LIMB_TO_RAM_TRANSPLANT,
            MMIO_INST_LIMB_TO_RAM_ONE_TARGET,
            MMIO_INST_LIMB_TO_RAM_TWO_TARGET,
            MMIO_INST_RAM_TO_LIMB_TRANSPLANT,
            MMIO_INST_RAM_TO_LIMB_ONE_SOURCE,
            MMIO_INST_RAM_TO_LIMB_TWO_SOURCE)
        .contains(this.instruction);
  }
}
