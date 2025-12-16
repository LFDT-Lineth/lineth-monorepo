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

import static net.consensys.linea.zktracer.exceptions.ExceptionUtils.*;
import static net.consensys.linea.zktracer.exceptions.ExceptionUtils.getProgramRDCFromStaticCallToCodeAccount;
import static net.consensys.linea.zktracer.module.hub.signals.TracedException.RETURN_DATA_COPY_FAULT;
import static org.junit.jupiter.api.Assertions.assertEquals;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.ToyAccount;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

/*
In this test, we trigger all subsets possible of exceptions (except stack exceptions) at the same time for RETURNDATACOPY.
List of the combinations tested below
RDCX & OOGX : RETURNDATACOPY
RDCX & MXPX : RETURNDATACOPY
Note : As MXPX is a subcase of OOGX, we don't test MXPX & OOGX
*/

@ExtendWith(UnitTestWatcher.class)
public class ReturnDataCopyTest extends TracerTestBase {
  @Test
  void rdcAndOogExceptionsReturnDataCopy(TestInfo testInfo) {
    boolean MXPX = true;
    boolean RDCX = true;
    final ToyAccount codeProviderAccount =
        getAccountForAddressWithBytecode(codeAddress, return32BytesFFBytecode);

    // We calculate gas cost without triggering RDCX, else no gas cost is calculated
    BytecodeCompiler programWithoutRdcx = getProgramRDCFromStaticCallToCodeAccount(!RDCX, !MXPX);
    BytecodeRunner bytecodeRunnerWithoutRdcx = BytecodeRunner.of(programWithoutRdcx.compile());
    long gasCostWithoutRdcx =
        bytecodeRunnerWithoutRdcx.runOnlyForGasCost(
            List.of(codeProviderAccount), chainConfig, testInfo);

    // We compute the final gas cost with RDCX and OOGX trigger
    // We trigger RDCX by adding 1 to RDS
    int gasCostAddOne = 3 + 3; // Push + ADD
    long gasCostWithRdcxAndOogx = gasCostWithoutRdcx + gasCostAddOne - 1; // trigger OOGX

    // We run the program with RDCX trigger and gasCost for OOGX
    BytecodeCompiler program = getProgramRDCFromStaticCallToCodeAccount(RDCX, !MXPX);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(gasCostWithRdcxAndOogx, List.of(codeProviderAccount), chainConfig, testInfo);

    // RDCX check happens before OOGX in tracer
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }

  @Test
  void rdcAndMxpExceptionsReturnDataCopy(TestInfo testInfo) {
    boolean MXPX = true;
    boolean RDCX = true;
    final ToyAccount codeProviderAccount =
        getAccountForAddressWithBytecode(codeAddress, return32BytesFFBytecode);

    // We prepare a program with RDCX and MXPX
    BytecodeCompiler program = getProgramRDCFromStaticCallToCodeAccount(RDCX, MXPX);

    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(List.of(codeProviderAccount), chainConfig, testInfo);

    // RDCX check happens before MXPX in tracer
    assertEquals(
        RETURN_DATA_COPY_FAULT,
        bytecodeRunner.getHub().lastUserTransactionSection().commonValues.tracedException());
  }
}
