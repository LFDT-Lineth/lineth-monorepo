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
import net.consensys.linea.testing.AddressCollisions;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction;
import net.consensys.linea.zktracer.module.txndata.transactions.UserTransaction.DominantCost;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.AccessListEntry;
import org.hyperledger.besu.datatypes.Address;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

@ExtendWith(UnitTestWatcher.class)
public class TrivialExecutionTests extends TracerTestBase {

  /** The 'to' address has empty byte code. The transaction does TX_SKIP */
  @ParameterizedTest
  @MethodSource({"testSource", "testSourceWithAllCollisionCases"})
  void txSkipTest(
      Bytes callData,
      boolean provideAccessList,
      DominantCost dominantCostPrediction,
      AddressCollisions collision,
      TestInfo testInfo) {
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(Bytes.EMPTY);
    AccessListEntry accessListEntry =
        new AccessListEntry(Address.fromHexString("0xABCD"), List.of());
    List<AccessListEntry> accessList = provideAccessList ? List.of(accessListEntry) : List.of();
    if (collision == AddressCollisions.NO_COLLISION) {
      bytecodeRunner.run(callData, accessList, chainConfig, testInfo);
    } else {
      bytecodeRunner.runWithAddressCollision(
          callData, accessList, collision, chainConfig, testInfo);
    }
    // Test blocks contain 4 transactions: 2 system transactions, 1 user transaction (the one we
    // created) and 1 noop transaction.
    if (isPostPrague(fork)) {
      UserTransaction userTransaction =
          (UserTransaction) bytecodeRunner.getHub().txnData().operations().get(2);
      Preconditions.checkArgument(userTransaction.getDominantCost() == dominantCostPrediction);
    }
  }

  /** The 'to' address has byte code equal to 0x00. The transaction does immediately stop. */
  @ParameterizedTest
  @MethodSource({"testSource", "testSourceWithAllCollisionCases"})
  void trivialCalleeTest(
      Bytes callData,
      boolean provideAccessList,
      DominantCost dominantCostPrediction,
      AddressCollisions collision,
      TestInfo testInfo) {
    BytecodeCompiler program = BytecodeCompiler.newProgram(chainConfig);
    program.op(OpCode.STOP);
    BytecodeRunner bytecodeRunner = BytecodeRunner.of(program.compile());
    AccessListEntry accessListEntry =
        new AccessListEntry(Address.fromHexString("0xABCD"), List.of());
    List<AccessListEntry> accessList = provideAccessList ? List.of(accessListEntry) : List.of();
    if (collision == AddressCollisions.NO_COLLISION) {
      bytecodeRunner.run(callData, accessList, chainConfig, testInfo);
    } else {
      bytecodeRunner.runWithAddressCollision(
          callData, accessList, collision, chainConfig, testInfo);
    }
    // Test blocks contain 4 transactions: 2 system transactions, 1 user transaction (the one we
    // created) and 1 noop transaction.
    if (isPostPrague(fork)) {
      UserTransaction userTransaction =
          (UserTransaction) bytecodeRunner.getHub().txnData().operations().get(2);
      Preconditions.checkArgument(userTransaction.getDominantCost() == dominantCostPrediction);
    }
  }

  static Stream<Arguments> testSource() {
    /*
    Here we change the callData (specifically the length and the CallDataSetting) to make comparingEffectiveRefundsVsFloorCost.result()
    become true in UserTransaction.comparingEffectiveRefundToFloorCostComputationRow.
     */
    List<Arguments> arguments = new ArrayList<>();

    // Case ALL_ZEROS.
    // The transaction execution cost (TX_SKIP) is 21000 + 2400 + 4*length.
    // The floor cost is 21000 + 10*length.
    // The threshold is floor cost > execution cost i.e., 6*length > 2400 i.e., length > 400.
    arguments.add(
        Arguments.of(
            buildCallData(CallDataSetting.ALL_ZEROS, true, 400),
            true,
            DominantCost.EXECUTION_COST_DOMINATES,
            AddressCollisions.NO_COLLISION));
    arguments.add(
        Arguments.of(
            buildCallData(CallDataSetting.ALL_ZEROS, true, 401),
            true,
            DominantCost.FLOOR_COST_DOMINATES,
            AddressCollisions.NO_COLLISION));

    // Case ALL_NON_ZEROS_EXCEPT_FOR_FIRST.
    // The transaction execution cost (TX_SKIP) is 21000 + 2400 + 16*length.
    // The floor cost is 21000 + 40*length.
    // The threshold is floor cost > execution cost i.e., 24*length > 2400 i.e., length > 100.
    arguments.add(
        Arguments.of(
            buildCallData(CallDataSetting.ALL_NON_ZEROS_EXCEPT_FOR_FIRST, false, 100),
            true,
            DominantCost.EXECUTION_COST_DOMINATES,
            AddressCollisions.NO_COLLISION));
    arguments.add(
        Arguments.of(
            buildCallData(CallDataSetting.ALL_NON_ZEROS_EXCEPT_FOR_FIRST, false, 101),
            true,
            DominantCost.FLOOR_COST_DOMINATES,
            AddressCollisions.NO_COLLISION));

    // Case ZEROS_AND_NON_ZEROS.
    // caveat for simplicity we consider even sizes.
    // The transaction execution cost (TX_SKIP) is 21000 + 2400 + (16+4)*length/2.
    // The floor cost is 21000 + (40+10)*length/2.
    // The threshold is floor cost > execution cost i.e., (25-10)*length > 2400 i.e., length > 160.
    arguments.add(
        Arguments.of(
            buildCallData(CallDataSetting.ZEROS_AND_NON_ZEROS, false, 160),
            true,
            DominantCost.EXECUTION_COST_DOMINATES,
            AddressCollisions.NO_COLLISION));
    arguments.add(
        Arguments.of(
            buildCallData(CallDataSetting.ZEROS_AND_NON_ZEROS, false, 162),
            true,
            DominantCost.FLOOR_COST_DOMINATES,
            AddressCollisions.NO_COLLISION));

    return arguments.stream();
  }

  static Stream<Arguments> testSourceWithAllCollisionCases() {
    /*
    Here we change the callData (specifically the length and the CallDataSetting) to make comparingEffectiveRefundsVsFloorCost.result()
    become true in UserTransaction.comparingEffectiveRefundToFloorCostComputationRow.
     */
    List<Arguments> arguments = new ArrayList<>();

    // Case ALL_ZEROS with potential collision.
    // This explores, among others, a case which blew up on mainnet, where a transaction had sender
    // == recipient,
    // non-empty call data, and a bug in the senderAddressCollision() case of TxSkipSection was
    // triggered.
    //
    // The transaction execution cost (TX_SKIP) is 21000 + 4.
    // The floor cost is 21000 + 10.

    for (AddressCollisions collisionCase : AddressCollisions.values()) {
      arguments.add(
          Arguments.of(
              buildCallData(CallDataSetting.ALL_ZEROS, true, 1),
              false,
              DominantCost.FLOOR_COST_DOMINATES,
              collisionCase));
    }
    return arguments.stream();
  }

  // Support enums and methods
  enum CallDataSetting {
    ALL_ZEROS,
    ZEROS_AND_NON_ZEROS,
    ALL_NON_ZEROS_EXCEPT_FOR_FIRST
  }

  static Bytes buildCallData(CallDataSetting callDataSetting, boolean startsWithZero, int length) {
    Preconditions.checkArgument(
        callDataSetting != CallDataSetting.ZEROS_AND_NON_ZEROS || length > 1,
        "length must be at least 2");
    return switch (callDataSetting) {
      case ALL_ZEROS -> Bytes.fromHexString("00".repeat(length));
      case ZEROS_AND_NON_ZEROS ->
          Bytes.fromHexString(
              (startsWithZero ? "0000" : "0100")
                  + "ff00".repeat((length - 2) / 2)
                  + (length % 2 == 0 ? "" : "ff"));
      case ALL_NON_ZEROS_EXCEPT_FOR_FIRST ->
          Bytes.fromHexString((startsWithZero ? "00" : "01") + "ff".repeat(length - 1));
    };
  }

  // Testing support method
  @Test
  void buildCallDataTest(TestInfo testInfo) {
    for (int size = 2; size <= 4; size++) {
      Preconditions.checkArgument(
          buildCallData(CallDataSetting.ZEROS_AND_NON_ZEROS, false, size).size() == size);
    }
  }
}
