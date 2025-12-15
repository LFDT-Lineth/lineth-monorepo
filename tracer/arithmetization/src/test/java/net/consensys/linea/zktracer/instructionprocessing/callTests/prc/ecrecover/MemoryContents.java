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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecrecover;

import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecadd.MemoryContents.RND;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecadd.MemoryContents.WORD_HEX_SIZE;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.framework.PrecompileCallMemoryContents;

/**
 * Memory for <b>ECRECOVER</b> testing will be made to contain inputs of the form
 *
 * <p><b>[ h | v | r | s ]</b>
 *
 * <p>where <b>h</b>, <b>v</b>, <b>r</b> and <b>s</b> are <b>32</b>-byte integers and with the
 * precise contents being dictated by the enum. Most values of {@link MemoryContents} values are
 * self-explanatory. We will test in particular:
 *
 * <p>- {@link #MALFORMED_AT_7f_BUT_SALVAGEABLE} for cases where s is correct <i>save for its final
 * byte</i> which would have to be <b>0x00</b> to be valid but instead is <b>0xff</b>; the point
 * being that if we provide a <b>call data size</b> of 127 then the signature becomes valid;
 *
 * <p>- {@link #INVALID_V} where v is neither 27 nor 28;
 *
 * <p>- {@link #BOUNDARY_R} and {@link #BOUNDARY_S}: cases where <b>r</b> or <b>s</b> are equal to
 * <b>0</b> or the <b>secp256k1n</b> prime
 */
public enum MemoryContents implements PrecompileCallMemoryContents {
  ZEROS,
  WELL_FORMED,
  MALFORMED_AT_7f_BUT_SALVAGEABLE,
  INVALID_V,
  BOUNDARY_R,
  BOUNDARY_S,
  RANDOM,
  MALLEABLE;

  boolean variant = false;

  public void switchVariant() {
    variant = !variant;
  }

  public static final EcRecoverTuple VALID_TUPLE_WHERE_THE_FINAL_BYTE_OF_S_IS_ZERO_1 =
      new EcRecoverTuple(
          "74657374",
          "27",
          "29DFBC75A9092AC090852F1385EDF63FB0424F0E76FE6235AA92DDC6125931F5",
          "37A787EEC7E3F12C0DF48461BEBDAD9880D6C4CF298F1D6F6CAC201F65C82B00");

  public static final EcRecoverTuple VALID_TUPLE_WHERE_THE_FINAL_BYTE_OF_S_IS_ZERO_2 =
      new EcRecoverTuple(
          "74657374",
          "28",
          "242043660FDE1E9ED3B653C9DBD4EE7AD931FDCC2969DFC750F92A2E14FF5B4A",
          "9E61F78441A49A3746E17E59FC626FC279FD1CCEA3448AAF984E7F4C9C873400");

  public static final EcRecoverTuple INVALID_V_TUPLE_1 =
      new EcRecoverTuple(
          "74657374",
          "29",
          "CE146C6A682C1A7A5E1B56548D985764F8FD1547734B889AC7D9D0208CE375AA",
          "E3A544E21BE92DD9A9AFE15015F2DF94E17E7090D7D164013E3806983A4D4E00");

  public static final EcRecoverTuple INVALID_V_TUPLE_2 =
      new EcRecoverTuple(
          "74657374",
          "30",
          "C2728E508D0EC7F821E12D9401D2D0C66C5D2CAB2C17094D77462E1A72D7A9A4",
          "CE521971A70CF4DEBD00DECDE04D523068142E73D52CA7E81BFEF76B0F79A100");

  public static final EcRecoverTuple EVM_CODES_EXAMPLE =
      new EcRecoverTuple(
          "456e9aea5e197a1f1af7a3e85a3212fa4049a3ba34c2289b4c860fc0b0c64ef3",
          "28",
          "9242685bf161793cc25603c231bc2f568eb630ea16aa137d2664ac8038825608",
          "4f8ae3bd7535248d0bd448298cc2e2071e56992d0774dc340c368ae950852ada");

  /**
   * {@link #EVM_CODES_EXAMPLE_MALLEABLE} is derived from {@link #EVM_CODES_EXAMPLE} via
   *
   * <p>- switching v to other value (27 ↔ 28)
   *
   * <p>- switching s to its opposite (s' ← {@link EcRecoverTuple#SECP_256_K1N} - s)
   */
  public static final EcRecoverTuple EVM_CODES_EXAMPLE_MALLEABLE =
      new EcRecoverTuple(
          "456e9aea5e197a1f1af7a3e85a3212fa4049a3ba34c2289b4c860fc0b0c64ef3",
          "27",
          "9242685bf161793cc25603c231bc2f568eb630ea16aa137d2664ac8038825608",
          "b0751c428acadb72f42bb7d6733d1df79c5843b9a7d3c407b39bd3a37fb11667");

  public static final EcRecoverTuple RANDOM_TUPLE =
      new EcRecoverTuple(
          RND.substring(0, WORD_HEX_SIZE),
          RND.substring(64, 64 + WORD_HEX_SIZE),
          RND.substring(128, 128 + WORD_HEX_SIZE),
          RND.substring(192, 192 + WORD_HEX_SIZE));

  /**
   * {@link #memoryContents} converts the {@link MemoryContents} into a {@link BytecodeCompiler}
   * (from which we will later extract a byte slice) containing "interesting" memory contents for a
   * <b>ECRECOVER</b> call.
   *
   * <p><b>Note.</b> Calling this method twice in a row on the same {@link MemoryContents}'s
   * generally results in two different outputs. Indeed, this method starts by switching the {@link
   * #variant}.
   *
   * @return
   */
  public BytecodeCompiler memoryContents(ChainConfig chainConfig) {

    // we switch with every call
    switchVariant();

    switch (this) {
      case ZEROS -> {
        final String ZERO_WORD = "00".repeat(WORD_SIZE);
        return new EcRecoverTuple(ZERO_WORD, ZERO_WORD, ZERO_WORD, ZERO_WORD)
            .memoryContents(false, chainConfig);
      }
      case WELL_FORMED -> {
        return variant
            ? VALID_TUPLE_WHERE_THE_FINAL_BYTE_OF_S_IS_ZERO_1.memoryContents(false, chainConfig)
            : VALID_TUPLE_WHERE_THE_FINAL_BYTE_OF_S_IS_ZERO_2.memoryContents(false, chainConfig);
      }
      case MALFORMED_AT_7f_BUT_SALVAGEABLE -> {
        return variant
            ? VALID_TUPLE_WHERE_THE_FINAL_BYTE_OF_S_IS_ZERO_1.memoryContents(true, chainConfig)
            : VALID_TUPLE_WHERE_THE_FINAL_BYTE_OF_S_IS_ZERO_2.memoryContents(true, chainConfig);
      }
      case INVALID_V -> {
        return variant
            ? INVALID_V_TUPLE_1.memoryContents(false, chainConfig)
            : INVALID_V_TUPLE_2.memoryContents(false, chainConfig);
      }
      case BOUNDARY_R -> {
        return variant
            ? VALID_TUPLE_WHERE_THE_FINAL_BYTE_OF_S_IS_ZERO_1
                .replaceR(true)
                .memoryContents(false, chainConfig)
            : VALID_TUPLE_WHERE_THE_FINAL_BYTE_OF_S_IS_ZERO_2
                .replaceR(false)
                .memoryContents(false, chainConfig);
      }
      case BOUNDARY_S -> {
        return variant
            ? VALID_TUPLE_WHERE_THE_FINAL_BYTE_OF_S_IS_ZERO_1
                .replaceS(true)
                .memoryContents(false, chainConfig)
            : VALID_TUPLE_WHERE_THE_FINAL_BYTE_OF_S_IS_ZERO_2
                .replaceS(false)
                .memoryContents(false, chainConfig);
      }
      case RANDOM -> {
        return RANDOM_TUPLE.memoryContents(false, chainConfig);
      }
      case MALLEABLE -> {
        return variant
            ? EVM_CODES_EXAMPLE.memoryContents(false, chainConfig)
            : EVM_CODES_EXAMPLE_MALLEABLE.memoryContents(false, chainConfig);
      }
      default -> throw new RuntimeException("Unknown MemoryContentsParameter");
    }
  }
}
