/*
 * Copyright ConsenSys Inc.
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

package net.consensys.linea.zktracer.module.ext;

import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class ExtTests extends TracerTestBase {

  @Test
  void extTrivialTest(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(0)
                .push(0)
                .push(0)
                .op(ADDMOD)
                .op(POP)
                .push(0)
                .push(0)
                .push(0)
                .op(MULMOD)
                .op(POP)
                .push(1)
                .push(1)
                .push(1)
                .op(ADDMOD)
                .op(POP)
                .push(1)
                .push(1)
                .push(1)
                .op(MULMOD)
                .op(POP)
                .push(2)
                .push(0)
                .push(2)
                .op(MULMOD)
                .op(POP)
                .compile())
        .run(chainConfig, testInfo);
  }

  @Test
  void oneRandomNonTrivial(TestInfo testInfo) {
    BytecodeRunner.of(
            BytecodeCompiler.newProgram(chainConfig)
                .push(
                    Bytes.fromHexString(
                        "0x67676776767AB6756AA367676776767AB6756AA367676776767AB6756AA3"))
                .push(Bytes.fromHexString("0x1234567890ABCDEF"))
                .push(Bytes.fromHexString("0x666aaa6660000000000001234567890AABBCCDDFAACACA"))
                .op(ADDMOD)
                .op(POP)
                .push(Bytes.fromHexString("0x0BADB0770BADB0770BADB0770BADB0770BADB077"))
                .push(
                    Bytes.fromHexString("0xC0FFEEC0FFEEC0FFEEC0FFEEC0FFEEC0FFEEC0FFEEC0FFEEC0FFEE"))
                .push(Bytes.fromHexString("0x07ACA07ACA07ACA07ACA07ACA07ACA"))
                .op(MULMOD)
                .op(POP)
                .compile())
        .run(chainConfig, testInfo);
  }
}
