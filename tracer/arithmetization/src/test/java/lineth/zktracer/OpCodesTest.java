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

package lineth.zktracer;

import lineth.UnitTestWatcher;
import lineth.reporting.TracerTestBase;
import lineth.testing.BytecodeCompiler;
import lineth.testing.BytecodeRunner;
import lineth.zktracer.opcode.InstructionFamily;
import lineth.zktracer.opcode.OpCode;
import lineth.zktracer.opcode.OpCodeData;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class OpCodesTest extends TracerTestBase {

  @Test
  public void AllOpCodesTest(TestInfo testInfo) {
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(getAllOpCodesProgram(testInfo));
    bytecodeRunner.run(chainConfig, testInfo);
  }

  private Bytes getAllOpCodesProgram(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    //
    for (OpCodeData opCodeData : opcodes.iterator()) {
      if (opCodeData != null) {
        if (opCodeData.instructionFamily() != InstructionFamily.HALT
            && opCodeData.instructionFamily() != InstructionFamily.JUMP) {
          OpCode opCode = opCodeData.mnemonic();
          int nPushes = opCodeData.stackSettings().delta();
          for (int i = 0; i < nPushes; i++) {
            program.push(0);
          }
          program.op(opCode);
          if (opCodeData.stackSettings().alpha() != 0) {
            program.op(OpCode.POP);
          }
        }
      }
    }
    return program.compile();
  }
}
