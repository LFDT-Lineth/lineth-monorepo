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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.modexp;

import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static net.consensys.linea.zktracer.opcode.OpCode.STATICCALL;

import java.util.ArrayList;
import java.util.List;

import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.*;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.params.provider.Arguments;

public class ParameterGeneration {

  /**
   * Generates test parameters for the happy path tests of <b>MODEXP</b>.
   *
   * <p>- {@code opCode} is one of the CALL-type {@link OpCode}'s
   *
   * <p>- {@code gas} is one of the {@link GasParameter} values (currently defaults to {@link
   * GasParameter#FULL})
   *
   * <p>- {@code bbs}, {@code ebs}, {@code mbs} are {@link ByteSizeParameter} values for
   * <b>MODEXP</b>, either 0, 1, something small, or the maximum (512)
   *
   * <p>- {@code cds} is one of the {@link CallDataSizeParameter} values, which dictates how much of
   * the parameters in RAM (<b>bbs</b>, <b>ebs</b>, <b>mbs</b>, <b>BASE</b>, <b>EXPONENT</b> and
   * <b>MODULUS</b>) actually get passed down to <b>MODEXP</b>
   *
   * <p>- {@code returnAt} is one of the {@link ReturnAtParameter} values, which dictates how much
   * of the return data will be written to RAM, it can be <b>EMPTY</b>, <b>PARTIAL</b> or
   * <b>FULL</b> in terms of the modulus byte size (<b>mbs</b>)
   *
   * <p>- {@code relPos} is one of the {@link RelativeRangePosition} values, which dictates whether
   * the various memory ranges for call data / return at etc ... ovrlap or not.
   *
   * @return Stream of test parameters
   */
  public static List<Arguments> parameterGeneration() {
    List<OpCode> CallOpCodes = List.of(CALL, CALLCODE, DELEGATECALL, STATICCALL);

    List<Arguments> argumentsList = new ArrayList<>();

    for (OpCode opCode : CallOpCodes) { // 4
      for (GasParameter gas : GasParameter.values()) { // 5
        for (ByteSizeParameter bbs : ByteSizeParameter.values()) { // 5
          for (ByteSizeParameter ebs : ByteSizeParameter.values()) { // 5
            for (ByteSizeParameter mbs : ByteSizeParameter.values()) { // 5
              for (CallDataSizeParameter cds : CallDataSizeParameter.values()) { // 9
                for (ReturnAtParameter returnAt : ReturnAtParameter.values()) { // 4
                  for (RelativeRangePosition relPos : RelativeRangePosition.values()) { // 2

                    argumentsList.add(
                        Arguments.of(
                            new CallParameters(
                                opCode,
                                gas,
                                new MemoryContents(bbs, ebs, mbs, cds),
                                returnAt,
                                relPos,
                                true)));

                    argumentsList.add(
                        Arguments.of(
                            new CallParameters(
                                opCode,
                                gas,
                                new MemoryContents(bbs, ebs, mbs, cds),
                                returnAt,
                                relPos,
                                false)));
                  }
                }
              }
            }
          }
        }
      }
    }
    return argumentsList;
  }
}
