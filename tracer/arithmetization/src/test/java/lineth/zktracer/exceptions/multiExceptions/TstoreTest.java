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
package lineth.zktracer.exceptions.multiExceptions;

import static lineth.zktracer.Trace.GAS_CONST_G_TRANSACTION;
import static lineth.zktracer.exceptions.ExceptionUtils.*;
import static lineth.zktracer.module.hub.signals.TracedException.STATIC_FAULT;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.List;
import lineth.UnitTestWatcher;
import lineth.reporting.TracerTestBase;
import lineth.testing.BytecodeCompiler;
import lineth.testing.BytecodeRunner;
import lineth.testing.ToyAccount;
import lineth.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/*
In this test, we trigger all subsets possible of exceptions (except stack exceptions) at the same time for transients opcodes.
List of the combinations tested below
STATIC & OOGX : TSTORE, TLOAD
 */
@ExtendWith(UnitTestWatcher.class)
public class TstoreTest extends TracerTestBase {

  @Test
  void staticAndOutOfGasExceptionsTStore(TestInfo testInfo) {
    BytecodeCompiler program;
    try {
      program = simpleProgram(OpCode.TSTORE);
    } catch (IllegalArgumentException e) {
      // TLOAD/TSTORE are not supported prior to Cancun fork
      return;
    }
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
