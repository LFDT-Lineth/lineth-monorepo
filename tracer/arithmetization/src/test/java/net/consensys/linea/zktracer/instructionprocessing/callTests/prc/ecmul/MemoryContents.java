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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecmul;

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecadd.MemoryContents.*;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallMemoryContents;
import org.apache.tuweni.bytes.Bytes;

/**
 * Enumerates the different memory contents parameters for a precompile call. Similarly to {@link
 * net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecadd.MemoryContents}, memory
 * content comprises a certain number of data words and one final EVM word where the <b>CALL</b>
 * instruction will be required to write its return data:
 *
 * <p><b>[ COORD_X | COORD_Y || MULITIPLIER || ff .. ff ] </b>
 */
public enum MemoryContents implements PrecompileCallMemoryContents {
  /** <b>[ ZEROS | ZEROS || ZEROS || ff .. ff ]</b> */
  ZEROS,

  /** <b>[ ZEROS | ZEROS || RAND || ff .. ff ]</b> */
  POINT_AT_INFINITY_RANDOM_MULTIPLIER,

  /** <b>[ A_x | A_y || ZEROS || ff .. ff ]</b> */
  CURVE_POINT_ZERO_MULTIPLIER,

  /** <b>[ 00 .. 00 ff | RAND || RAND || ff .. ff ]</b> */
  MALFORMED_AT_1f,

  /** <b>[ ZEROS | 00 .. 00 ff || RAND || ff .. ff ]</b> */
  MALFORMED_AT_3f,

  /** <b>[ B_x | B_y || 00 .. 00 xx .. xx || ff .. ff ]</b> with {@code nByte} many xx's */
  FIRST_FEW_BYTES_OF_MULTIPLIER_ARE_ZERO,

  /** <b>[ C_x | C_y || RAND || ff .. ff ]</b> */
  WELL_FORMED_POINT_AND_NONTRIVIAL_MULTIPLIER,

  /** <b>[ RAND | RAND || RAND || ff .. ff ]</b> */
  RANDOM;

  public boolean variant = false;

  public void switchVariant() {
    variant = !variant;
  }

  public BytecodeCompiler memoryContents(ChainConfig chainConfig) {
    int nBytes = 11;
    String pointData =
        switch (this) {
          case ZEROS -> ZERO_WORD.repeat(3);
          case POINT_AT_INFINITY_RANDOM_MULTIPLIER ->
              ZERO_WORD.repeat(2) + RND.substring(17, 17 + WORD_HEX_SIZE);
          case CURVE_POINT_ZERO_MULTIPLIER -> (variant ? A_X + A_Y : B_X + B_Y) + ZERO_WORD;
          case MALFORMED_AT_1f -> MAX_BYTE + RND.substring(61, 61 + 2 * WORD_HEX_SIZE);
          case MALFORMED_AT_3f -> ZERO_WORD + MAX_BYTE + RND.substring(129, 129 + WORD_HEX_SIZE);
          case FIRST_FEW_BYTES_OF_MULTIPLIER_ARE_ZERO ->
              (variant ? B_X + B_Y : C_X + C_Y)
                  + "00".repeat(nBytes)
                  + RND.substring(99, 99 + 2 * (WORD_SIZE - nBytes));
          case WELL_FORMED_POINT_AND_NONTRIVIAL_MULTIPLIER ->
              (variant ? C_X + C_Y : D_X + D_Y) + RND.substring(71, 71 + WORD_HEX_SIZE);
          case RANDOM -> RND.substring(36, 36 + 3 * WORD_HEX_SIZE);
        };

    checkState(
        pointData.length() == 3 * WORD_HEX_SIZE,
        "ECADD memory contents: point data size %s is not the expected one %s",
        pointData.length(),
        3 * WORD_HEX_SIZE);

    String memoryContentsString = pointData + MAX_WORD;

    BytecodeCompiler memoryContents = BytecodeCompiler.newProgram(chainConfig);
    memoryContents.immediate(Bytes.fromHexString(memoryContentsString));

    return memoryContents;
  }
}
