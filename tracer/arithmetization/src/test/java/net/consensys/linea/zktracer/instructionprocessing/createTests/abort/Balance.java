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
package net.consensys.linea.zktracer.instructionprocessing.createTests.abort;

import static net.consensys.linea.zktracer.instructionprocessing.createTests.SizeParameter.*;
import static net.consensys.linea.zktracer.instructionprocessing.createTests.trivial.RootLevel.*;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.Calls.appendRevert;
import static net.consensys.linea.zktracer.opcode.OpCode.SHA3;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.zktracer.instructionprocessing.createTests.CreateType;
import net.consensys.linea.zktracer.instructionprocessing.createTests.OffsetParameter;
import net.consensys.linea.zktracer.instructionprocessing.createTests.SizeParameter;
import net.consensys.linea.zktracer.instructionprocessing.createTests.ValueParameter;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class Balance extends TracerTestBase {

  /** The tests below are meant to trigger the "insufficientBalanceAbort" condition. */
  @ParameterizedTest
  @MethodSource("rootLevelInsufficientBalanceParameters")
  void rootLevelInsufficientBalanceCreateOpcodeTest(
      CreateType createType,
      SizeParameter sizeParameter,
      OffsetParameter offsetParameter,
      TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    if (sizeParameter == s_MSIZE) {
      program.push(513).push(0).op(SHA3); // purely to expand memory to 0 < 512 + 32 bytes
    }

    genericCreate(
        program,
        createType,
        ValueParameter.v_SELFBALANCE_PLUS_ONE,
        offsetParameter,
        sizeParameter,
        salt01);

    run(program, chainConfig, testInfo);
  }

  /**
   * In the tests {@link #rootLevelAbortThenSuccessCreateTest} we perform two CREATE(2)'s in a row.
   * The first one is designed to raise the insufficientBalanceAbort condition, the second one is
   * designed to succeed. We optionally REVERT.
   */
  @ParameterizedTest
  @MethodSource("offsetAndSizeParameters")
  void rootLevelAbortThenSuccessCreateTest(
      CreateType createType,
      OffsetParameter offsetParameter,
      SizeParameter sizeParameter,
      boolean reverts,
      TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    genericCreate(
        program,
        createType,
        ValueParameter.v_SELFBALANCE_PLUS_ONE,
        offsetParameter,
        sizeParameter,
        salt01); // aborts
    genericCreate(
        program, createType, ValueParameter.v_ONE, offsetParameter, sizeParameter, salt01);

    if (reverts) {
      appendRevert(program, 2, 13);
    }

    run(program, chainConfig, testInfo);
  }

  @Test
  void specialRootLevelAbortThenSuccessCreateTest(TestInfo testInfo) {
    final CreateType createType = CreateType.CREATE;
    final OffsetParameter offsetParameter = OffsetParameter.o_ZERO;
    final SizeParameter sizeParameter = SizeParameter.s_ZERO;
    final boolean reverts = true;

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    genericCreate(
        program,
        createType,
        ValueParameter.v_SELFBALANCE_PLUS_ONE,
        offsetParameter,
        sizeParameter,
        salt01); // aborts
    genericCreate(
        program, createType, ValueParameter.v_ONE, offsetParameter, sizeParameter, salt01);

    if (reverts) {
      appendRevert(program, 2, 13);
    }

    run(program, chainConfig, testInfo);
  }

  @Test
  void rootLevelAbortThenSuccessCreateTest(TestInfo testInfo) {

    // parameters
    CreateType createType = CreateType.CREATE;
    OffsetParameter offsetParameter = OffsetParameter.o_ZERO;
    SizeParameter sizeParameter = SizeParameter.s_ZERO;
    boolean reverts = true;

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    genericCreate(
        program,
        createType,
        ValueParameter.v_SELFBALANCE_PLUS_ONE,
        offsetParameter,
        sizeParameter,
        salt01); // aborts
    genericCreate(
        program, createType, ValueParameter.v_ONE, offsetParameter, sizeParameter, salt01);

    if (reverts) {
      appendRevert(program, 2, 13);
    }

    run(program, chainConfig, testInfo);
  }

  /**
   * In the tests {@link #rootLevelSuccessThenAbortCreateTest} we perform two CREATE(2)'s in a row.
   * The first one is designed to succeed, the second one is designed to raise the
   * insufficientBalanceAbort condition. We optionally REVERT.
   */
  @ParameterizedTest
  @MethodSource("offsetAndSizeParameters")
  void rootLevelSuccessThenAbortCreateTest(
      CreateType createType,
      OffsetParameter offsetParameter,
      SizeParameter sizeParameter,
      boolean reverts,
      TestInfo testInfo) {
    BytecodeCompiler program =
        rootLevelSuccessThenAbortCreateByteCodeCompiler(
            createType, offsetParameter, sizeParameter, reverts);
    run(program, chainConfig, testInfo);
  }

  @Test
  void specificRootLevelSuccessThenAbortCreateTest(TestInfo testInfo) {
    BytecodeCompiler program =
        rootLevelSuccessThenAbortCreateByteCodeCompiler(
            CreateType.CREATE2, OffsetParameter.o_ZERO, SizeParameter.s_ZERO, false);
    run(program, chainConfig, testInfo);
  }

  BytecodeCompiler rootLevelSuccessThenAbortCreateByteCodeCompiler(
      CreateType createType,
      OffsetParameter offsetParameter,
      SizeParameter sizeParameter,
      boolean reverts) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    genericCreate(
        program, createType, ValueParameter.v_ONE, offsetParameter, sizeParameter, salt01);
    genericCreate(
        program,
        createType,
        ValueParameter.v_SELFBALANCE_PLUS_ONE,
        offsetParameter,
        sizeParameter,
        salt01); // aborts
    if (reverts) {
      appendRevert(program, 2, 13);
    }
    return program;
  }

  private static Stream<Arguments> offsetAndSizeParameters() {
    List<Arguments> arguments = new ArrayList<>();
    for (OffsetParameter offsetParameter : OffsetParameter.values()) {
      for (SizeParameter sizeParameter : SizeParameter.values()) {
        if (sizeParameter.willRaiseException()) continue;
        arguments.add(Arguments.of(CreateType.CREATE, offsetParameter, sizeParameter, true));
        arguments.add(Arguments.of(CreateType.CREATE2, offsetParameter, sizeParameter, false));
      }
    }
    return arguments.stream();
  }

  /**
   * {@link #rootLevelInsufficientBalanceParameters} excludes ''large sizes'': we are interested in
   * unexceptional but aborted CREATE(2)'s.
   */
  private static Stream<Arguments> rootLevelInsufficientBalanceParameters() {

    List<Arguments> arguments = new ArrayList<>();
    List<SizeParameter> sizeParameters =
        List.of(s_ZERO, s_TWELVE, s_THIRTEEN, s_FOURTEEN, s_THIRTY_TWO, s_MSIZE);

    for (CreateType createType : CreateType.values()) {
      for (OffsetParameter offsetParameter : OffsetParameter.values()) {
        for (SizeParameter sizeParameter : sizeParameters) {
          arguments.add(Arguments.of(createType, sizeParameter, offsetParameter));
        }
      }
    }

    return arguments.stream();
  }
}
