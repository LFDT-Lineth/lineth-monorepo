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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.p256verify;

import static net.consensys.linea.zktracer.Fork.isPostOsaka;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_G_CALL_STIPEND;
import static net.consensys.linea.zktracer.Trace.GAS_CONST_P256_VERIFY;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY;
import static net.consensys.linea.zktracer.Trace.PRECOMPILE_RETURN_DATA_SIZE___P256_VERIFY;
import static net.consensys.linea.zktracer.instructionprocessing.callTests.Utilities.simpleCallAndReturnDataSize;
import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertTrue;

import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.common.postCancun.fixedSizeFixedGasCost.P256VerifyOobCall;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class P256VerifyCallSuccessAndFailureCasesTest extends TracerTestBase {

  @Test
  void insufficientGasP256VerifyCall_ExpectedCallFailure(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    simpleCallAndReturnDataSize(
        program,
        STATICCALL,
        GAS_CONST_P256_VERIFY - 1,
        Address.P256_VERIFY,
        0,
        0,
        0,
        0,
        PRECOMPILE_RETURN_DATA_SIZE___P256_VERIFY);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
    if (isPostOsaka(fork)) {
      P256VerifyOobCall p256VerifyOobCall =
          (P256VerifyOobCall)
              bytecodeRunner.getHub().oob().operations().stream().toList().getLast();
      assertFalse(p256VerifyOobCall.isHubSuccess());
      final Bytes returnDataSize = bytecodeRunner.getHub().currentFrame().frame().getStackItem(0);
      assertTrue(returnDataSize.isZero());
    }
  }

  @Test
  void sufficientGasWithStipendP256VerifyCall_ExpectedCallSuccess(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    simpleCallAndReturnDataSize(
        program,
        CALL,
        GAS_CONST_P256_VERIFY - GAS_CONST_G_CALL_STIPEND,
        Address.P256_VERIFY,
        1,
        0,
        0,
        0,
        PRECOMPILE_RETURN_DATA_SIZE___P256_VERIFY);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
    if (isPostOsaka(fork)) {
      P256VerifyOobCall p256VerifyOobCall =
          (P256VerifyOobCall)
              bytecodeRunner.getHub().oob().operations().stream().toList().getLast();
      assertTrue(p256VerifyOobCall.isHubSuccess());
    }
  }

  @Test
  void sufficientGasAndInvalidCDSP256VerifyCall_ExpectedCallSuccess(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    simpleCallAndReturnDataSize(
        program,
        STATICCALL,
        GAS_CONST_P256_VERIFY,
        Address.P256_VERIFY,
        0,
        0,
        0,
        0,
        PRECOMPILE_RETURN_DATA_SIZE___P256_VERIFY);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
    if (isPostOsaka(fork)) {
      P256VerifyOobCall p256VerifyOobCall =
          (P256VerifyOobCall)
              bytecodeRunner.getHub().oob().operations().stream().toList().getLast();
      assertTrue(p256VerifyOobCall.isHubSuccess());
    }
  }

  @Test
  void sufficientGasAndValidCDSP256VerifyCall_ExpectedCallSuccess(TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    simpleCallAndReturnDataSize(
        program,
        STATICCALL,
        GAS_CONST_P256_VERIFY,
        Address.P256_VERIFY,
        0,
        0,
        PRECOMPILE_CALL_DATA_SIZE___P256_VERIFY,
        0,
        PRECOMPILE_RETURN_DATA_SIZE___P256_VERIFY);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    bytecodeRunner.run(chainConfig, testInfo);
    if (isPostOsaka(fork)) {
      P256VerifyOobCall p256VerifyOobCall =
          (P256VerifyOobCall)
              bytecodeRunner.getHub().oob().operations().stream().toList().getLast();
      assertTrue(p256VerifyOobCall.isHubSuccess());
    }
  }
}
