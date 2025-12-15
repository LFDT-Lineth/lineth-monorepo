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

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecadd.MemoryContents.MAX_WORD;

import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.ChainConfig;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

public record EcRecoverTuple(String h, String v, String r, String s) {

  public static String SECP_256_K1N =
      "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141";
  public static String ZERO = "00".repeat(WORD_SIZE);

  /**
   * {@link #memoryContents} converts the {@link EcRecoverTuple} into the byte slice
   *
   * <p><b>[ h | v | r | s | ff .. ff ]</b>
   *
   * <p>and optionally on {@code changeFinalByteOfS} modifies the final byte of <b>s</b>. As per
   * usual, the purpose of the last EVM word is to be overwritten by return data.
   *
   * @param changeFinalByteOfS
   * @return
   */
  public BytecodeCompiler memoryContents(boolean changeFinalByteOfS, ChainConfig chainConfig) {
    Bytes hBytes = Bytes32.leftPad(Bytes.fromHexString(h));
    Bytes vBytes = Bytes32.leftPad(Bytes.fromHexString(v));
    Bytes rBytes = Bytes32.leftPad(Bytes.fromHexString(r));
    Bytes sBytes = Bytes32.leftPad(Bytes.fromHexString(s));

    if (changeFinalByteOfS) {
      sBytes = sBytes.xor(Bytes32.leftPad(Bytes.ofUnsignedLong(0xff)));
    }

    Bytes pointData =
        Bytes.concatenate(hBytes, vBytes, rBytes, sBytes, Bytes.fromHexString(MAX_WORD));

    checkState(
        pointData.size() == 5 * WORD_SIZE,
        "ECRECOVER input vector size %s is not the expected one %s",
        pointData.size(),
        5 * WORD_SIZE);

    BytecodeCompiler memoryContents = BytecodeCompiler.newProgram(chainConfig);
    memoryContents.immediate(pointData);

    return memoryContents;
  }

  /**
   * {@link #replaceR} produces a copy of {@code this} with {@link #r} replaced by either
   *
   * <p>- {@link #ZERO}
   *
   * <p>- {@link #SECP_256_K1N}
   *
   * @param useZero
   * @return
   */
  public EcRecoverTuple replaceR(boolean useZero) {
    return new EcRecoverTuple(h, v, zeroOrPrime(useZero), s);
  }

  /**
   * {@link #replaceS} produces a copy of {@code this} with {@link #s} replaced by either
   *
   * <p>- {@link #ZERO}
   *
   * <p>- {@link #SECP_256_K1N}
   *
   * @param useZero
   * @return
   */
  public EcRecoverTuple replaceS(boolean useZero) {
    return new EcRecoverTuple(h, v, r, zeroOrPrime(useZero));
  }

  private String zeroOrPrime(boolean returnZero) {
    return returnZero ? ZERO : SECP_256_K1N;
  }
}
