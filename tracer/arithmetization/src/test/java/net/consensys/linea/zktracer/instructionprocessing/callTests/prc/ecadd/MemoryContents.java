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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecadd;

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE_MO;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallMemoryContents;
import org.apache.tuweni.bytes.Bytes;

public enum MemoryContents implements PrecompileCallMemoryContents {
  ZEROS,
  WELL_FORMED_POINTS,
  MIXED,
  MALFORMED_AT_1f,
  MALFORMED_AT_3f,
  MALFORMED_AT_5f,
  MALFORMED_AT_7f,
  RANDOM,
  MAX;

  boolean variant = false;

  // coordinates for 5 curve points
  public static final String A_X =
      "08d555288636cfb5abeac1d38a828fc4d975fb8def9f63a34f8c91701b5478d1";
  public static final String A_Y =
      "2fe58eb05ff7bed143c398b0b62e18e8b0327a3be8250b2923ea29e6773a65f5";
  public static final String B_X =
      "0d89e2e42be7fbda9358a2689c73af3ecd519728359f175ee7919d31c8f61d5d";
  public static final String B_Y =
      "0eb7c8cfbbe0a89bf12697e97b482c3a91ff985ba456f1684a0b68efa2933019";
  public static final String C_X =
      "070375d4eec4f22aa3ad39cb40ccd73d2dbab6de316e75f81dc2948a996795d5";
  public static final String C_Y =
      "041b98f07f44aa55ce8bd97e32cacf55f1e42229d540d5e7a767d1138a5da656";
  public static final String D_X =
      "185f6f5cf93c8afa0461a948c2da7c403b6f8477c488155dfa8d2da1c62517b8";
  public static final String D_Y =
      "13d83d7a51eb18fdb51225873c87d44f883e770ce2ca56c305d02d6cb99ca5b8";
  public static final String RND =
      "e2db57e640f49001c04ca5cb36e72f97af535c4d7620a48b96f8d0475afcaee569dcf211255b9ce6c05178cdf45152650496523591db85dadc328f6cb57e94ad83a66cca880b9fc02154c6941457158585230a843f38778f1d4cd6cbb42c778bcc5f05ab1c8306b59db726b705e3f782017a4dcaa04694b5c62e645445ede56b";

  public static final String ZERO_WORD = "00".repeat(WORD_SIZE);
  public static final String MAX_BYTE = "00".repeat(WORD_SIZE_MO) + "ff";
  public static final String MAX_WORD = "ff".repeat(WORD_SIZE);
  public static final int WORD_HEX_SIZE = 2 * WORD_SIZE;

  @Override
  public void switchVariant() {
    variant = !variant;
  }

  /**
   * Constructs a slice of bytes of the following form
   *
   * <p><b>[ W_1 | W_2 | W_3 | W_4 | ff .. ff ]</b>
   *
   * <p>for various EVM words <b>W_k</b>. These may or may not contain x/y coordinates of curve
   * points. The final <b>32</b> bytes are all set to <b>ff</b> and <i>may</i> get overwritten with
   * return data after the precompile call.
   *
   * <p>Wrt the EVM words <b>W_k</b>, there are several cases: coordinates of actual curve points,
   * coordinates of the form <b>00 .. 00 ff</b> that don't match the other coordinate, <b>00 .. 00
   * 00</b>, just random bytes ...
   *
   * <p><b>Note.</b> The purpose of the resulting slice of bytes is to become the 'byte code' of
   * some account whose 'byte code' is to be <b>EXTCODECOPY</b>'ed to memory and used as input to a
   * call to <b>ECADD</b>.
   */
  public BytecodeCompiler memoryContents(ChainConfig chainConfig) {

    // Note that 4 = 2 * 2. We need 4 * 32 hex characters for the data representing a point.
    String pointData =
        switch (this) {
          case ZEROS -> ZERO_WORD.repeat(4);
          case WELL_FORMED_POINTS -> variant ? A_X + A_Y + B_X + B_Y : C_X + C_Y + D_X + D_Y;
          case MIXED ->
              variant
                  ? A_X + A_Y + RND.substring(13, 13 + 2 * WORD_HEX_SIZE)
                  : C_X + C_Y + RND.substring(99, 99 + 2 * WORD_HEX_SIZE);
          case MALFORMED_AT_1f -> MAX_BYTE + (variant ? A_Y + B_X + B_Y : C_Y + D_X + D_Y);
          case MALFORMED_AT_3f -> ZERO_WORD + MAX_BYTE + (variant ? B_X + B_Y : D_X + D_Y);
          case MALFORMED_AT_5f -> ZERO_WORD.repeat(2) + MAX_BYTE + (variant ? B_Y : D_Y);
          case MALFORMED_AT_7f -> ZERO_WORD.repeat(3) + MAX_BYTE;
          case RANDOM -> RND;
          case MAX -> MAX_WORD.repeat(4);
        };

    checkState(
        pointData.length() == 4 * WORD_HEX_SIZE,
        "ECADD memory contents: point data size %s is not the expected one %s",
        pointData.length(),
        4 * WORD_HEX_SIZE);

    String memoryContentsString = pointData + MAX_WORD;

    BytecodeCompiler memoryContents = BytecodeCompiler.newProgram(chainConfig);
    memoryContents.immediate(Bytes.fromHexString(memoryContentsString));

    return memoryContents;
  }
}
