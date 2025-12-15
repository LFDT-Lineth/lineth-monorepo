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

import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static net.consensys.linea.zktracer.opcode.OpCode.STATICCALL;

import java.util.ArrayList;
import java.util.List;

import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.GasParameter;
import net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ReturnAtParameter;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.junit.jupiter.params.provider.Arguments;

public class ParameterGeneration {

  /**
   * Parameter generation for <b>ECADD</b> testing. The paramters encompass the following:
   *
   * <p>- <b>OpCode</b> the call opcode to be tested; the only distinction we draw here is between
   * value-bearing <b>CALL</b>-type opcodes and non-value-bearing ones; the value bearing ones are
   * interesting in conjunction with <b>ZERO_WORD</b> gas calls to <b>ECADD</b>;
   *
   * <p>- <b>GasParameter</b> the gas parameter to be tested: the relevant options are
   * <b>ZERO_WORD</b>, <b>COST_MO</b>, <b>COST</b>, and <b>FULL</b>; recall that the gas cost of
   * <b>ECADD</b> is constant equal to 150; testing with <b>ZERO_WORD</b> gas makes sense for value
   * bearing <b>CALL</b>-type opcodes given the call stipend of <b>2_300</b>;
   *
   * <p>- <b>MemoryContentParameter</b> the memory content parameter to be tested; the way we
   * produce this memory content is by providing 5 EVM words, the last one of which is blackened out
   * (i.e. set to <b>0x ff .. ff ff</b>), the others being either coordinates of curve points,
   * invalid coordinates of the form <b>0x 00 .. 00 ff</b>, random junk;
   *
   * <p>- <b>CallDataSizeParameter</b> the call data size parameter to be tested; options are
   * <b>EMPTY</b>, some number of full EVM words with the final one either full (e.g.
   * <b>NONEMPTY_20</b>) or missing the final byte (e.g. <b>NONEMPTY_5f</b>), and <b>FULL</b>;
   *
   * <p>- <b>ReturnAtParameter</b> the return at parameter to be tested; return data will be written
   * on the aforementioned blackened word in RAM;
   */
  public static List<Arguments> parameterGeneration() {
    List<OpCode> CallOpCodes = List.of(CALL, CALLCODE, DELEGATECALL, STATICCALL);
    List<GasParameter> GasParameters =
        List.of(GasParameter.ZERO, GasParameter.COST_MO, GasParameter.COST, GasParameter.PLENTY);
    List<ReturnAtParameter> ReturnAtParameters =
        List.of(ReturnAtParameter.EMPTY, ReturnAtParameter.PARTIAL, ReturnAtParameter.FULL);

    List<Arguments> argumentsList = new ArrayList<>();

    for (OpCode opCode : CallOpCodes) { // 4
      for (GasParameter gas : GasParameters) { // 4
        for (MemoryContents memoryContent : MemoryContents.values()) { // 9
          for (CallDataSizeParameter cds : CallDataSizeParameter.values()) { // 10
            for (ReturnAtParameter returnAt : ReturnAtParameters) { // 3

              argumentsList.add(
                  Arguments.of(
                      new CallParameters(opCode, gas, memoryContent, cds, returnAt, true)));

              argumentsList.add(
                  Arguments.of(
                      new CallParameters(opCode, gas, memoryContent, cds, returnAt, false)));
            }
          }
        }
      }
    }
    return argumentsList;
  }
}
