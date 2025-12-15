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
package net.consensys.linea.zktracer.exceptions;

import static net.consensys.linea.zktracer.Trace.WORD_SIZE;
import static net.consensys.linea.zktracer.opcode.OpCode.*;

import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.hub.Hub;
import org.apache.tuweni.bytes.Bytes;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

/**
 * We explore what happens if the first instruction after resuming the execution of a frame is
 * exceptional. We also explore what happens, though this ought not to be an issue, what happens
 * when resuming the execution of the parent frame after an exception in the child frame. This tests
 * whether {@link Hub#unlatchStack} behaves appropriately for <b>CALL</b>-type and
 * <b>CREATE</b>-type instructions, when either
 *
 * <ul>
 *   <li>the child frame finishes on an exception [yes/no]
 *   <li>the first instruction in the parent frame is exceptional
 * </ul>
 *
 * <p><b>Note.</b> All exceptions that we test are <b>stackUnderflowException</b>'s.
 */
public class ExceptionAtContextReEntryTest extends TracerTestBase {

  /**
   * {@link #firstInstructionAfterResumingFromUnsuccessfulMessageCallIsExceptional} tests what
   * happens if both
   *
   * <ul>
   *   <li>a STATICCALL ends in an exception
   *   <li>the first instruction after the parent resumes execution is exceptional, too
   * </ul>
   *
   * <b>Note.</b> Both exceptions are <b>stackUnderflowException</b>'s.
   */
  @Test
  public void firstInstructionAfterResumingFromUnsuccessfulMessageCallIsExceptional(
      TestInfo testInfo) {

    BytecodeCompiler initCode = BytecodeCompiler.newProgram(chainConfig);
    initCode.push(ADD.byteValue()).push(0).op(MSTORE8).push(1).push(0).op(RETURN);
    // this init code deploys the byte code "0x01" (when given sufficient gas)

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    // we do a deployment of a smart contract whose bytecode is "0x01"
    Bytes initCodeBytes = initCode.compile();

    program.push(initCodeBytes).push(0).op(MSTORE);
    program.push(initCodeBytes.size()).push(WORD_SIZE - initCodeBytes.size()).push(0).op(CREATE);
    // should leave the stack containing
    // | deploymentAddress ]

    final int storageKey = 255;
    program.push(storageKey).op(SSTORE);

    // message call to a simple smart contract
    program
        .push(0) // r@c
        .push(0) // r@c
        .push(0) // r@c
        .push(0) // r@c
        .push(storageKey)
        .op(SLOAD) // deployment address
        .push(0xff) // gas
        .op(STATICCALL);

    // the previous operation leaves the stack in the following state
    // | b ], where b ∈ {0, 1}, here: 0
    // i.e. containing precisely one (boolean) item. The ADD instruction will therefore raise
    // a stackUnderflowException.
    program.op(ADD);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  /**
   * {@link #firstInstructionAfterResumingFromSuccessfulMessageCallIsExceptional} tests what happens
   * if both
   *
   * <ul>
   *   <li>a STATICCALL ends in an exception
   *   <li>the first instruction after the parent resumes execution is exceptional, too
   * </ul>
   *
   * <b>Note.</b> Both exceptions are <b>stackUnderflowException</b>'s.
   */
  @Test
  public void firstInstructionAfterResumingFromSuccessfulMessageCallIsExceptional(
      TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);

    // message call to a simple smart contract
    program
        .push(0) // r@c
        .push(0) // r@c
        .push(0) // r@c
        .push(0) // r@c
        .push("c0de") // address
        .push(0xff) // gas
        .op(STATICCALL);

    // the previous operation leaves the stack in the following state
    // | b ], where b ∈ {0, 1}
    // i.e. containing precisely one (boolean) item. The ADD instruction will therefore raise
    // a stackUnderflowException.
    program.op(ADD);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  /**
   * {@link #firstInstructionAfterResumingFromSuccessfulContractCreationIsExceptional} tests what
   * happens if the first instruction after a successful deployment is exceptional.
   *
   * <p><b>Note.</b> The exceptions is a <b>stackUnderflowException</b>.
   */
  @Test
  public void firstInstructionAfterResumingFromSuccessfulContractCreationIsExceptional(
      TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program.push(0).push(0).push(0).op(CREATE);

    // the previous operation leaves the stack in the following state
    // | a ], where a is either 0 or the deployment address, in the present case: the deployment
    // address
    // i.e. containing precisely one item. The ADD instruction will therefore raise a
    // stackUnderflowException.
    program.op(ADD);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }

  /**
   * {@link #firstInstructionAfterResumingFromUnsuccessfulContractCreationIsExceptional} tests what
   * happens if the first instruction after an unsuccessful deployment is exceptional, too.
   *
   * <p><b>Note.</b> The exceptions is a <b>stackUnderflowException</b>.
   */
  @Test
  public void firstInstructionAfterResumingFromUnsuccessfulContractCreationIsExceptional(
      TestInfo testInfo) {

    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program.push(ADD.byteValue()).push(0).op(MSTORE8);
    // memory = [01 00 00 ... [

    program.push(1).push(0).push(0).op(CREATE);
    // the init code is "0x01", which will cause an exception in the CREATE

    // the previous operation leaves the stack in the following state
    // | a ], where a is either 0 or the deployment address, in the present case: ZERO
    // i.e. containing precisely one item. The ADD instruction will therefore raise a
    // stackUnderflowException.
    program.op(ADD);

    BytecodeRunner.of(program.compile()).run(chainConfig, testInfo);
  }
}
