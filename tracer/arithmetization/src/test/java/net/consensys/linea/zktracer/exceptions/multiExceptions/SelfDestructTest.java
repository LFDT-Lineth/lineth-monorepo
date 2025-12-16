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

package net.consensys.linea.zktracer.exceptions.multiExceptions;

import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_TRANSACTION;
import static net.consensys.linea.zktracer.exceptions.ExceptionUtils.*;
import static net.consensys.linea.zktracer.exceptions.ExceptionUtils.getProgramStaticCallToCodeAddress;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.STATIC_FAULT;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/*
In this test, we trigger all subsets possible of exceptions (except stack exceptions) at the same time for SELFDESTRUCT opcode.
List of the combinations tested below
STATIC & OOGX : SELFDESTRUCT
 */

@ExtendWith(UnitTestWatcher.class)
public class SelfDestructTest extends TracerTestBase {

  @Test
  void staticAndOogExceptionsSelfDestruct(TestInfo testInfo) {

    BytecodeCompiler program = simpleProgram(OpCode.SELFDESTRUCT);
    Bytes pgCompile = program.compile();
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(pgCompile);
    long gasCostTx = bytecodeRunner.runOnlyForGasCost(chainConfig, testInfo);

    int cornerCase = -1;
    // We calculate gas cost to trigger OOGX
    int gasCostMinusCornerCase = (int) gasCostTx - GAS_CONST_G_TRANSACTION + cornerCase;

    // We prepare a program with a static call to code account
    ToyAccount codeProviderAccount = getAccountForAddressWithBytecode(codeAddress, pgCompile);
    BytecodeCompiler pgStaticCallToCode = getProgramStaticCallToCodeAddress(gasCostMinusCornerCase);

    // Run with linea block gas limit so gas cost is passed to child without 63/64
    BytecodeRunner bytecodeRunnerStaticCall = BytecodeRunner.of(pgStaticCallToCode.compile());
    bytecodeRunnerStaticCall.run(List.of(codeProviderAccount), chainConfig, testInfo);

    // Static check happens before OOGX in tracer
    assertEquals(
        STATIC_FAULT,
        bytecodeRunnerStaticCall
            .getHub()
            .lastUserTransactionSection(2)
            .commonValues
            .tracedException());
  }
}
