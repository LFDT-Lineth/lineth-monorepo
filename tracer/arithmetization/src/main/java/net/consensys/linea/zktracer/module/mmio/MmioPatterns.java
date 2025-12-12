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

import static net.consensys.linea.zktracer.Trace.LLARGE;

import java.util.ArrayList;
import java.util.List;

import net.consensys.linea.zktracer.module.mmu.MmuData;
import net.consensys.linea.zktracer.types.Bytes16;
import org.apache.tuweni.bytes.Bytes;

public class MmioPatterns {

  static List<Bytes> isolateSuffix(final Bytes16 input, final List<Boolean> flag) {
    final List<Bytes> output = new ArrayList<>(LLARGE);

    output.addFirst(flag.getFirst() ? Bytes.of(input.get(0)) : Bytes.EMPTY);

    for (short ct = 1; ct < LLARGE; ct++) {
      output.add(
          ct,
          flag.get(ct)
              ? Bytes.concatenate(output.get(ct - 1), Bytes.of(input.get(ct)))
              : output.get(ct - 1));
    }

    return output;
  }

  static List<Bytes> isolatePrefix(final Bytes16 input, final List<Boolean> flag) {
    final List<Bytes> output = new ArrayList<>(LLARGE);

    output.addFirst(flag.getFirst() ? Bytes.EMPTY : Bytes.of(input.get(0)));

    for (short ct = 1; ct < LLARGE; ct++) {
      output.add(
          ct,
          flag.get(ct)
              ? output.get(ct - 1)
              : Bytes.concatenate(output.get(ct - 1), Bytes.of(input.get(ct))));
    }

    return output;
  }

  static List<Bytes> isolateChunk(
      final Bytes16 input, final List<Boolean> startFlag, final List<Boolean> endFlag) {
    final List<Bytes> output = new ArrayList<>(LLARGE);

    output.addFirst(startFlag.getFirst() ? Bytes.of(input.get(0)) : Bytes.EMPTY);

    for (short ct = 1; ct < LLARGE; ct++) {
      if (startFlag.get(ct)) {
        output.add(
            ct,
            endFlag.get(ct)
                ? output.get(ct - 1)
                : Bytes.concatenate(output.get(ct - 1), Bytes.of(input.get(ct))));
      } else {
        output.add(ct, Bytes.EMPTY);
      }
    }

    return output;
  }

  static List<Bytes> power(final List<Boolean> flag) {
    final List<Bytes> output = new ArrayList<>(LLARGE);

    output.addFirst(flag.getFirst() ? Bytes.ofUnsignedShort(256) : Bytes.of(1));

    for (short ct = 1; ct < LLARGE; ct++) {
      final Bytes toPut =
          flag.get(ct) ? Bytes.concatenate(output.get(ct - 1), Bytes.of(0)) : output.get(ct - 1);
      output.add(
          ct,
          flag.get(ct) ? Bytes.concatenate(output.get(ct - 1), Bytes.of(0)) : output.get(ct - 1));
    }
    return output;
  }

  static List<Bytes> antiPower(final List<Boolean> flag) {
    final List<Bytes> output = new ArrayList<>(LLARGE);

    output.addFirst(flag.getFirst() ? Bytes.of(1) : Bytes.ofUnsignedShort(256));

    for (short ct = 1; ct < LLARGE; ct++) {
      output.add(
          ct,
          flag.get(ct) ? output.get(ct - 1) : Bytes.concatenate(output.get(ct - 1), Bytes.of(0)));
    }
    return output;
  }

  static boolean plateau(int m, int counter) {
    return counter >= m;
  }

  public static Bytes16 onePartialToOne(
      final Bytes16 source,
      final Bytes16 target,
      final short sourceByteOffset,
      final short targetByteOffset,
      final short size) {
    return Bytes16.wrap(
        Bytes.concatenate(
            target.slice(0, targetByteOffset),
            source.slice(sourceByteOffset, size),
            target.slice(targetByteOffset + size, LLARGE - targetByteOffset - size)));
  }

  public static Bytes16 onePartialToTwoOutputOne(
      final Bytes16 source,
      final Bytes16 target1,
      final short sourceByteOffset,
      final short targetByteOffset) {
    return Bytes16.wrap(
        Bytes.concatenate(
            target1.slice(0, targetByteOffset),
            source.slice(sourceByteOffset, LLARGE - targetByteOffset)));
  }

  public static Bytes16 onePartialToTwoOutputTwo(
      final Bytes16 source,
      final Bytes16 target2,
      final short sourceByteOffset,
      final short targetByteOffset,
      final short size) {
    final short numberOfBytesFromSourceToFirstTarget = (short) (LLARGE - targetByteOffset);
    final short numberOfBytesFromSourceToSecondTarget =
        (short) (size - numberOfBytesFromSourceToFirstTarget);
    return Bytes16.wrap(
        Bytes.concatenate(
            source.slice(
                sourceByteOffset + numberOfBytesFromSourceToFirstTarget,
                numberOfBytesFromSourceToSecondTarget),
            target2.slice(
                numberOfBytesFromSourceToSecondTarget,
                LLARGE - numberOfBytesFromSourceToSecondTarget)));
  }

  public static Bytes16 twoPartialToOne(
      final Bytes16 source1,
      final Bytes16 source2,
      final Bytes16 target,
      final short sourceByteOffset,
      final short targetByteOffset,
      final short size) {
    final short numberByteFromFirstSource = (short) (LLARGE - sourceByteOffset);
    final short numberByteFromSecondSource = (short) (size - numberByteFromFirstSource);
    return Bytes16.wrap(
        Bytes.concatenate(
            target.slice(0, targetByteOffset),
            source1.slice(sourceByteOffset, numberByteFromFirstSource),
            source2.slice(0, numberByteFromSecondSource),
            target.slice(targetByteOffset + size)));
  }

  public static Bytes16 excision(
      final Bytes16 target, final short targetByteOffset, final short size) {
    return Bytes16.wrap(
        Bytes.concatenate(
            target.slice(0, targetByteOffset),
            Bytes.repeat((byte) 0, size),
            target.slice(targetByteOffset + size)));
  }

  public static void updateTemporaryTargetRam(
      MmuData mmuData, final long targetLimbOffsetToUpdate, final Bytes16 newLimb) {
    final Bytes bytesPreLimb =
        Bytes.repeat(
            (byte) 0,
            (int)
                (LLARGE
                    * targetLimbOffsetToUpdate)); // We won't access the preLimb again, so we don't
    // care
    // of its value
    final Bytes bytesPostLimb =
        mmuData.targetRamBytes().slice((int) ((targetLimbOffsetToUpdate + 1) * LLARGE));

    mmuData.targetRamBytes(Bytes.concatenate(bytesPreLimb, newLimb, bytesPostLimb));
  }
}
