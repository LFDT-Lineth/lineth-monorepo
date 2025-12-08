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
package net.consensys.linea.zktracer.instructionprocessing.callTests;

import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.eoaAddress;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.untrimmedEoaAddress;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class TrimmingTests extends TracerTestBase {

  @Test
  void targetTrimming(TestInfo testInfo) {

    BytecodeCompiler bytecode =
        BytecodeCompiler.newProgram(chainConfig)
            .push(1)
            .push(2)
            .push(3)
            .push(4)
            .push(0) // value
            .push(untrimmedEoaAddress) // address
            .push(0) // gas
            .op(CALL)
            .op(POP)
            .push(untrimmedEoaAddress) // address
            .op(BALANCE)
            .push(eoaAddress) // address
            .op(BALANCE)
            .push(1)
            .push(2)
            .push(3)
            .push(4)
            .push(0) // value
            .push(eoaAddress) // address
            .push(0) // gas
            .op(CALL)
            .op(POP)
            .push(untrimmedEoaAddress) // address
            .op(BALANCE)
            .push(eoaAddress) // address
            .op(EXTCODEHASH);

    BytecodeRunner.of(bytecode).run(chainConfig, testInfo);
  }
}
