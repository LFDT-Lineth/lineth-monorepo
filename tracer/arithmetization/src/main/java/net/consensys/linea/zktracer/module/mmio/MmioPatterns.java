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

import static graphql.com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.LLARGE;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.zktracer.module.mmu.MmuData;
import org.apache.tuweni.bytes.Bytes;

public class MmioPatterns {

  static List<Bytes> isolateSuffix(final Bytes input, final List<Boolean> flag) {
    checkArgument(input.size() == LLARGE, "input should be of size 16");
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

  static List<Bytes> isolatePrefix(final Bytes input, final List<Boolean> flag) {
    checkArgument(input.size() == LLARGE, "input should be of size 16");
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
      final Bytes input, final List<Boolean> startFlag, final List<Boolean> endFlag) {
    checkArgument(input.size() == LLARGE, "input should be of size 16");
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

  public static Bytes onePartialToOne(
      final Bytes source,
      final Bytes target,
      final short sourceByteOffset,
      final short targetByteOffset,
      final short size) {
    checkArgument(source.size() == LLARGE, "source should be of size 16");
    checkArgument(target.size() == LLARGE, "target should be of size 16");
    final Bytes output =
        Bytes.concatenate(
            target.slice(0, targetByteOffset),
            source.slice(sourceByteOffset, size),
            target.slice(targetByteOffset + size, LLARGE - targetByteOffset - size));
    checkArgument(output.size() == LLARGE, "output should be of size 16");
    return output;
  }

  public static Bytes onePartialToTwoOutputOne(
      final Bytes source,
      final Bytes target1,
      final short sourceByteOffset,
      final short targetByteOffset) {
    checkArgument(source.size() == LLARGE, "source should be of size 16");
    checkArgument(target1.size() == LLARGE, "target1 should be of size 16");
    final Bytes output =
        Bytes.concatenate(
            target1.slice(0, targetByteOffset),
            source.slice(sourceByteOffset, LLARGE - targetByteOffset));
    checkArgument(output.size() == LLARGE, "output should be of size 16");
    return output;
  }

  public static Bytes onePartialToTwoOutputTwo(
      final Bytes source,
      final Bytes target2,
      final short sourceByteOffset,
      final short targetByteOffset,
      final short size) {
    checkArgument(source.size() == LLARGE, "source should be of size 16");
    checkArgument(target2.size() == LLARGE, "target2 should be of size 16");
    final short numberOfBytesFromSourceToFirstTarget = (short) (LLARGE - targetByteOffset);
    final short numberOfBytesFromSourceToSecondTarget =
        (short) (size - numberOfBytesFromSourceToFirstTarget);
    final Bytes output =
        Bytes.concatenate(
            source.slice(
                sourceByteOffset + numberOfBytesFromSourceToFirstTarget,
                numberOfBytesFromSourceToSecondTarget),
            target2.slice(
                numberOfBytesFromSourceToSecondTarget,
                LLARGE - numberOfBytesFromSourceToSecondTarget));
    checkArgument(output.size() == LLARGE, "output should be of size 16");
    return output;
  }

  public static Bytes twoPartialToOne(
      final Bytes source1,
      final Bytes source2,
      final Bytes target,
      final short sourceByteOffset,
      final short targetByteOffset,
      final short size) {
    checkArgument(source1.size() == LLARGE, "source1 should be of size 16");
    checkArgument(source2.size() == LLARGE, "source2 should be of size 16");
    checkArgument(target.size() == LLARGE, "target should be of size 16");
    final short numberByteFromFirstSource = (short) (LLARGE - sourceByteOffset);
    final short numberByteFromSecondSource = (short) (size - numberByteFromFirstSource);
    final Bytes output =
        Bytes.concatenate(
            target.slice(0, targetByteOffset),
            source1.slice(sourceByteOffset, numberByteFromFirstSource),
            source2.slice(0, numberByteFromSecondSource),
            target.slice(targetByteOffset + size));
    checkArgument(output.size() == LLARGE, "output should be of size 16");
    return output;
  }

  public static Bytes excision(final Bytes target, final short targetByteOffset, final short size) {
    checkArgument(target.size() == LLARGE, "target should be of size 16");
    final Bytes output =
        Bytes.concatenate(
            target.slice(0, targetByteOffset),
            Bytes.repeat((byte) 0, size),
            target.slice(targetByteOffset + size));
    checkArgument(output.size() == LLARGE, "output should be of size 16");
    return output;
  }

  public static void updateTemporaryTargetRam(
      MmuData mmuData, final long targetLimbOffsetToUpdate, final Bytes newLimb) {
    checkArgument(newLimb.size() == LLARGE, "newLimb should be of size 16");
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
