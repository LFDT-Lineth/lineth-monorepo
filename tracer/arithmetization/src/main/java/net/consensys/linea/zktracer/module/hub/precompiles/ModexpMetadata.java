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

package net.consensys.linea.zktracer.module.hub.precompiles;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.TraceOsaka.EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND;
import static net.consensys.linea.zktracer.module.Util.rightPaddedSlice;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.*;
import static net.consensys.linea.zktracer.types.Conversions.bytesToShort;
import static net.consensys.linea.zktracer.types.Conversions.safeLongToInt;
import static net.consensys.linea.zktracer.types.Utils.rightPadTo;

import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase;
import net.consensys.linea.zktracer.types.EWord;
import net.consensys.linea.zktracer.types.MemoryRange;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.internal.Words;

@Getter
@Accessors(fluent = true)
public final class ModexpMetadata {
  public static final int BBS_MIN_OFFSET = 0x00;
  public static final int EBS_MIN_OFFSET = 0x20;
  public static final int MBS_MIN_OFFSET = 0x40;
  public static final int BASE_MIN_OFFSET = 0x60;
  private static int MODEXP_LARGE_INPUT_BYTE_WIDTH = 32;

  private final MemoryRange callDataRange;
  @Setter private Bytes rawResult;

  public short getNumberOfRowsForModexp() {
    return (short) (4 * (getMaxInputSize() / LLARGE));
  }

  public int getMaxInputSize() {
    return EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND;
  }

  public Bytes normalize(ModexpXbsCase modexpXbsCase) {
    return tracedIsWithinBounds(modexpXbsCase) ? xbs(modexpXbsCase).toBytes() : Bytes.EMPTY;
  }

  public Bytes normalizedBbs() {
    return normalize(MODEXP_XBS_CASE_BBS);
  }

  public short normalizedBbsInt() {
    final short result = bytesToShort(normalizedBbs());
    checkArgument(
        ((0 <= result) && result <= getMaxInputSize()), "Normalized BBS out of bounds: " + result);
    return result;
  }

  public Bytes normalizedEbs() {
    return normalize(MODEXP_XBS_CASE_EBS);
  }

  public short normalizedEbsInt() {
    final short result = bytesToShort(normalizedEbs());
    checkArgument(
        ((0 <= result) && result <= getMaxInputSize()), "Normalized EBS out of bounds: " + result);
    return result;
  }

  public Bytes normalizedMbs() {
    return normalize(MODEXP_XBS_CASE_MBS);
  }

  public ModexpMetadata(MemoryRange callDataRange) {
    this.callDataRange = callDataRange;
  }

  public Bytes callData() {
    return callDataRange.extract();
  }

  public boolean extractBbs() {
    return callData().size() > BBS_MIN_OFFSET;
  }

  public boolean extractEbs() {
    return callData().size() > EBS_MIN_OFFSET;
  }

  public boolean extractMbs() {
    return callData().size() > MBS_MIN_OFFSET;
  }

  public Bytes rawBbs() {
    return rightPaddedSlice(callData(), BBS_MIN_OFFSET, WORD_SIZE);
  }

  public Bytes rawEbs() {
    return rightPaddedSlice(callData(), EBS_MIN_OFFSET, WORD_SIZE);
  }

  public Bytes rawMbs() {
    return rightPaddedSlice(callData(), MBS_MIN_OFFSET, WORD_SIZE);
  }

  private int bbsShift() {
    return EBS_MIN_OFFSET - Math.min(EBS_MIN_OFFSET, callData().size());
  }

  public EWord bbs() {
    return EWord.of(rawBbs().shiftRight(bbsShift()).shiftLeft(bbsShift()));
  }

  private int ebsShift() {
    return extractEbs()
        ? EBS_MIN_OFFSET - Math.min(EBS_MIN_OFFSET, callData().size() - EBS_MIN_OFFSET)
        : 0;
  }

  public EWord ebs() {
    return EWord.of(rawEbs().shiftRight(ebsShift()).shiftLeft(ebsShift()));
  }

  private int mbsShift() {
    return extractMbs()
        ? EBS_MIN_OFFSET - Math.min(EBS_MIN_OFFSET, callData().size() - MBS_MIN_OFFSET)
        : 0;
  }

  public EWord mbs() {
    return EWord.of(rawMbs().shiftRight(mbsShift()).shiftLeft(mbsShift()));
  }

  public EWord xbs(ModexpXbsCase modexpXbsCase) {
    return switch (modexpXbsCase) {
      case MODEXP_XBS_CASE_BBS -> bbs();
      case MODEXP_XBS_CASE_EBS -> ebs();
      case MODEXP_XBS_CASE_MBS -> mbs();
    };
  }

  public int bbsInt() {
    return Words.clampedToInt(bbs());
  }

  public int ebsInt() {
    return Words.clampedToInt(ebs());
  }

  public int mbsInt() {
    return Words.clampedToInt(mbs());
  }

  public boolean loadRawLeadingWord() {
    return callData().size() > BASE_MIN_OFFSET + normalizedBbsInt() && normalizedEbsInt() != 0;
  }

  public boolean extractModulus() {
    return (callData().size() > BASE_MIN_OFFSET + bbsInt() + ebsInt()) && !mbs().isZero();
  }

  public boolean extractBase() {
    return extractModulus() && !bbs().isZero();
  }

  public boolean extractExponent() {
    return extractModulus() && !ebs().isZero();
  }

  public Bytes base() {
    Bytes unpadded = Bytes.EMPTY;
    final int firstOffset = BASE_MIN_OFFSET;
    if (callData().size() > firstOffset) {
      final int sizeToExtract = Math.min(bbsInt(), callData().size() - firstOffset);
      unpadded = callData().slice(BASE_MIN_OFFSET, sizeToExtract);
    }
    return rightPadTo(unpadded, bbsInt());
  }

  public Bytes exp() {
    Bytes unpadded = Bytes.EMPTY;
    final int firstOffset = BASE_MIN_OFFSET + bbsInt();
    if (callData().size() > firstOffset) {
      final int sizeToExtract = Math.min(ebsInt(), callData().size() - firstOffset);
      unpadded = callData().slice(BASE_MIN_OFFSET + bbsInt(), sizeToExtract);
    }
    return rightPadTo(unpadded, ebsInt());
  }

  public Bytes mod() {
    Bytes unpadded = Bytes.EMPTY;
    final int firstOffset = BASE_MIN_OFFSET + bbsInt() + ebsInt();
    if (callData().size() > firstOffset) {
      final int sizeToExtract = Math.min(mbsInt(), callData().size() - firstOffset);
      unpadded = callData().slice(firstOffset, sizeToExtract);
    }
    return rightPadTo(unpadded, (int) Words.clampedToLong(mbs()));
  }

  public boolean mbsNonZero() {
    return !mbs().isZero();
  }

  public EWord rawLeadingWord() {
    return EWord.of(
        rightPaddedSlice(
            callDataRange.getRawData(),
            safeLongToInt(callDataRange.offset()) + BASE_MIN_OFFSET + normalizedBbsInt(),
            WORD_SIZE));
  }

  /** This is to detect large (ie > 32 bytes = 256 bit) modexp for the prover */
  public boolean largeModexp() {
    return bbsInt() > MODEXP_LARGE_INPUT_BYTE_WIDTH
        || ebsInt() > MODEXP_LARGE_INPUT_BYTE_WIDTH
        || mbsInt() > MODEXP_LARGE_INPUT_BYTE_WIDTH;
  }

  public boolean tracedIsWithinBounds(ModexpXbsCase modexpXbsCase) {
    return (Words.clampedToInt(xbs(modexpXbsCase)) <= getMaxInputSize());
  }

  public boolean tracedIsOutOfBounds(ModexpXbsCase modexpXbsCase) {
    return !tracedIsWithinBounds(modexpXbsCase);
  }

  public boolean allXbsesAreInBounds() {
    return tracedIsWithinBounds(MODEXP_XBS_CASE_BBS)
        && tracedIsWithinBounds(MODEXP_XBS_CASE_EBS)
        && tracedIsWithinBounds(MODEXP_XBS_CASE_MBS);
  }
}
