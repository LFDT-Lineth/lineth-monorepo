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

package net.consensys.linea.testing;

import java.util.ArrayList;
import java.util.LinkedList;
import java.util.List;
import java.util.Random;
import java.util.function.BiConsumer;
import java.util.stream.Stream;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.module.add.Add;
import net.consensys.linea.zktracer.module.ext.Ext;
import net.consensys.linea.zktracer.module.mod.Mod;
import net.consensys.linea.zktracer.module.mul.Mul;
import net.consensys.linea.zktracer.module.mxp.Mxp;
import net.consensys.linea.zktracer.module.shf.Shf;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes32;
import org.junit.jupiter.api.DynamicTest;
import org.junit.jupiter.api.TestInfo;

/**
 * Responsible for executing JUnit 5 dynamic tests for modules with the ability to configure custom
 * opcode arguments on a test case basis.
 */
public class DynamicTests {
  private static final Random RAND = new Random();
  private static final int TEST_REPETITIONS = 4;

  private final List<DynamicTestCase> testCaseRegistry;

  private final Module module;

  private DynamicTests(Module module) {
    this.module = module;
    this.testCaseRegistry = new LinkedList<>();
  }

  /**
   * Constructor function for initialization of a suite of dynamic tests per module.
   *
   * @param module module instance for which a dynamic test suite should be run
   * @return an instance of {@link DynamicTests}
   */
  public static DynamicTests forModule(Module module) {
    return new DynamicTests(module);
  }

  /**
   * Constructs a new dynamically generated test case.
   *
   * @param testCaseName name of the test case
   * @param args arguments of the test case
   * @return the current instance
   */
  public DynamicTests testCase(final String testCaseName, final List<OpcodeCall> args) {
    testCaseRegistry.add(new DynamicTestCase(testCaseName, args, null));

    return this;
  }

  /**
   * Constructs a new dynamically generated test case.
   *
   * @param testCaseName name of the test case
   * @param args arguments of the test case
   * @param customAssertions custom test case assertions
   * @return the current instance
   */
  public DynamicTests testCase(
      final String testCaseName,
      final List<OpcodeCall> args,
      final BiConsumer<OpCode, List<Bytes32>> customAssertions) {
    testCaseRegistry.add(new DynamicTestCase(testCaseName, args, customAssertions));

    return this;
  }

  /**
   * Runs the suite of dynamic tests per module.
   *
   * @return a {@link Stream} of {@link DynamicTest} ran by a {@link
   *     org.junit.jupiter.api.TestFactory}
   */
  public Stream<DynamicTest> run(ChainConfig chainConfig, TestInfo testInfo) {
    return this.testCaseRegistry.stream()
        .flatMap(
            e ->
                generateTestCases(
                    e.name(), e.arguments(), e.customAssertions(), chainConfig, testInfo));
  }

  private List<OpCode> supportedOpCodes(Module module) {
    if (module instanceof Add) {
      return List.of(OpCode.ADD, OpCode.SUB);
    } else if (module instanceof Ext) {
      return List.of(OpCode.MULMOD, OpCode.ADDMOD);
    } else if (module instanceof Mod) {
      return List.of(OpCode.DIV, OpCode.SDIV, OpCode.MOD, OpCode.SMOD);
    } else if (module instanceof Mul) {
      return List.of(OpCode.MUL, OpCode.EXP);
    } else if (module instanceof Shf) {
      return List.of(OpCode.SHR, OpCode.SHL, OpCode.SAR);
    } else if (module instanceof Wcp) {
      return List.of(OpCode.LT, OpCode.GT, OpCode.SLT, OpCode.SGT, OpCode.EQ, OpCode.ISZERO);
    } else if (module instanceof Mxp) {
      return List.of(
          OpCode.SHA3,
          OpCode.LOG0,
          OpCode.LOG1,
          OpCode.LOG2,
          OpCode.LOG3,
          OpCode.LOG4,
          OpCode.RETURN,
          OpCode.REVERT,
          OpCode.MSIZE,
          OpCode.CALLDATACOPY,
          OpCode.CODECOPY,
          OpCode.RETURNDATACOPY,
          OpCode.EXTCODECOPY,
          OpCode.MLOAD,
          OpCode.MSTORE,
          OpCode.MSTORE8,
          OpCode.CREATE,
          OpCode.CREATE2,
          OpCode.CALL,
          OpCode.CALLCODE,
          OpCode.DELEGATECALL,
          OpCode.STATICCALL);
    } else {
      throw new RuntimeException("Unexpected module");
    }
  }

  /**
   * Abstracts away argument generation per module's supported opcodes.
   *
   * @param argsGenerationFunc consumer allowing for argument generation specification per module's
   *     supported opcode
   * @return a multimap of generated arguments for the given test case
   */
  public List<OpcodeCall> newModuleArgumentsProvider(
      final BiConsumer<List<OpcodeCall>, OpCode> argsGenerationFunc) {
    List<OpcodeCall> arguments = new ArrayList<>();

    for (OpCode opCode : supportedOpCodes(module)) {
      argsGenerationFunc.accept(arguments, opCode);
    }

    return arguments;
  }

  private Stream<DynamicTest> generateTestCases(
      final String testCaseName,
      final List<OpcodeCall> args,
      final BiConsumer<OpCode, List<Bytes32>> customAssertions,
      ChainConfig chainConfig,
      TestInfo testInfo) {
    return args.stream()
        .map(
            e -> {
              String testName =
                  "[%s][%s] Test bytecode for opcode %s with opArgs %s"
                      .formatted(
                          module.moduleKey().toString().toUpperCase(),
                          testCaseName,
                          e.opCode(),
                          e.args());

              return DynamicTest.dynamicTest(
                  testName,
                  () -> {
                    if (customAssertions == null) {
                      ModuleTests.runTestWithOpCodeArgs(
                          e.opCode(), e.args(), chainConfig, testInfo);
                    } else {
                      customAssertions.accept(e.opCode(), e.args());
                    }
                  });
            });
  }
}
