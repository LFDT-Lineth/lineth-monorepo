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

package net.consensys.linea.zktracer.forkSpecific.prague.floorprice;

import static net.consensys.linea.zktracer.Fork.isPostPrague;

import com.google.common.base.Preconditions;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.AccessListEntry;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class NontrivialExecutionTests extends TracerTestBase {

  static final Bytes callData = Bytes.fromHexString("0000");

  /**
   * The 'to' address has byte code which can be padded with low-cost opcodes. This allows us to aim
   * for the threshold where the floor price is overtaken by the execution cost.
   */
  @ParameterizedTest
  @MethodSource("adjustableByteCodeTestSource")
  void adjustableByteCodeTest(
      Bytes byteCode,
      boolean provideAccessList,
      UserTransaction.DominantCost dominantCostPrediction,
      TestInfo testInfo) {
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(byteCode);
    AccessListEntry accessListEntry =
        new AccessListEntry(Address.fromHexString("0xABCD"), List.of());
    List<AccessListEntry> accessList = provideAccessList ? List.of(accessListEntry) : List.of();
    bytecodeRunner.run(callData, accessList, chainConfig, testInfo);
    // Test blocks contain 4 transactions: 2 system transactions, 1 user transaction (the one we
    // created) and 1 noop transaction.
    if (isPostPrague(fork)) {
      UserTransaction userTransaction =
          (UserTransaction) bytecodeRunner.getHub().txnData().operations().get(2);
      Preconditions.checkArgument(userTransaction.getDominantCost() == dominantCostPrediction);
    }
  }

  static Stream<Arguments> adjustableByteCodeTestSource() {
    List<Arguments> arguments = new ArrayList<>();

    arguments.add(
        Arguments.of(
            buildProgram(
                TransactionCategory.MESSAGE_CALL,
                UserTransaction.DominantCost.FLOOR_COST_DOMINATES),
            false,
            UserTransaction.DominantCost.FLOOR_COST_DOMINATES));
    arguments.add(
        Arguments.of(
            buildProgram(
                TransactionCategory.MESSAGE_CALL,
                UserTransaction.DominantCost.EXECUTION_COST_DOMINATES),
            false,
            UserTransaction.DominantCost.EXECUTION_COST_DOMINATES));

    return arguments.stream();
  }

  /**
   * The 'init code' is a sequence of JUMPDEST. Every byte contributes
   *
   * <ul>
   *   <li>40 to the floor cost
   *   <li>16 to the upfront gas cost
   *   <li>1 to the execution cost
   * </ul>
   *
   * Furthermore, every word of the init code contributes 2 to the upfront gas cost.
   *
   * <p>The threshold is given by the following equation (ics = init_code_size):
   *
   * <pre><b>21_000 + 40*ics > 21_000 + 32_000 + 16*ics + 2*⌈ics/32⌉ + 1*ics</b></pre>
   *
   * which puts the threshold somewhere around 1390-1400 bytes. This allows us to aim for the
   * threshold where the floor price is overtaken by the execution cost.
   */
  @ParameterizedTest
  @MethodSource("adjustableInitCodeTestSource")
  void adjustableInitCodeTest(
      Bytes initCode, UserTransaction.DominantCost dominantCostPrediction, TestInfo testInfo) {
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(initCode);
    bytecodeRunner.runInitCode(chainConfig, testInfo);
    // Test blocks contain 4 transactions: 2 system transactions, 1 user transaction (the one we
    // created) and 1 noop transaction.
    if (isPostPrague(fork)) {
      UserTransaction userTransaction =
          (UserTransaction) bytecodeRunner.getHub().txnData().operations().get(2);
      Preconditions.checkArgument(userTransaction.getDominantCost() == dominantCostPrediction);
    }
  }

  static Stream<Arguments> adjustableInitCodeTestSource() {
    List<Arguments> arguments = new ArrayList<>();

    arguments.add(
        Arguments.of(
            buildProgram(
                TransactionCategory.DEPLOYMENT, UserTransaction.DominantCost.FLOOR_COST_DOMINATES),
            UserTransaction.DominantCost.FLOOR_COST_DOMINATES));
    arguments.add(
        Arguments.of(
            buildProgram(
                TransactionCategory.DEPLOYMENT,
                UserTransaction.DominantCost.EXECUTION_COST_DOMINATES),
            UserTransaction.DominantCost.EXECUTION_COST_DOMINATES));

    return arguments.stream();
  }

  // Support enums and methods
  enum TransactionCategory {
    DEPLOYMENT,
    MESSAGE_CALL
  }

  private static Bytes buildProgram(
      TransactionCategory transactionCategory, UserTransaction.DominantCost dominantCost) {
    return switch (transactionCategory) {
      case MESSAGE_CALL -> {
        /**
         * Given the {@link callData} the floor cost is 21_000 + 2*10 and the execution cost is
         * 21_000 + 2*4 + 1*codesize. The threshold is thus reached when floor cost > execution cost
         * i.e., 21_000 + 2*10 > 21_000 + 2*4 + 1*codesize that is codesize < 2*(10-4) = 12
         */
        BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
        program.op(
            OpCode.JUMPDEST,
            dominantCost == UserTransaction.DominantCost.EXECUTION_COST_DOMINATES ? 12 : 11);
        yield program.compile();
      }
      case DEPLOYMENT -> {
        /**
         * Given the {@link callData} the floor cost is 21_000 + init_code_size*40 (all bytes will
         * be nonzero) and the execution cost is 21_000 + 32_000 + 2*4 + 2*1 + code_execution_cost.
         */
        BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
        program.op(
            OpCode.JUMPDEST,
            dominantCost == UserTransaction.DominantCost.EXECUTION_COST_DOMINATES ? 1395 : 1396);
        yield program.compile();
      }
    };
  }
}
