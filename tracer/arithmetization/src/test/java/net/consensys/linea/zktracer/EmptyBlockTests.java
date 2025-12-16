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

package net.consensys.linea.zktracer;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.zktracer.Fork.isPostCancun;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;
import static org.junit.jupiter.api.parallel.ExecutionMode.SAME_THREAD;

import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.MultiBlockExecutionEnvironment;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.module.hub.section.LogSection;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.module.hub.state.State;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.parallel.Execution;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.ValueSource;

// Adding this annotation to ensure tests run in the same thread, avoiding potential concurrency
// issues
// As most tests will run with a Besu node
@Execution(SAME_THREAD)
public class EmptyBlockTests extends TracerTestBase {

  enum BlockType {
    EMPTY_BLOCK,
    MONO_TRANSACTION_BLOCK___STORING,
    MONO_TRANSACTION_BLOCK___READING,
  }

  final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
  final Address senderAddress =
      Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));

  final ToyAccount senderAccount =
      ToyAccount.builder().balance(Wei.fromEth(128)).nonce(5).address(senderAddress).build();

  final ToyAccount receivingAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .nonce(116)
          .address(Address.fromHexStringStrict("0x1122334455667788990011223344556677889900"))
          .code(
              BytecodeCompiler.newProgram(chainConfig)
                  .push(0) // ret size
                  .push(0) // ret offset
                  .push(0) // size
                  .push(0) // offset

                  // address:
                  .op(OpCode.CALLDATASIZE) // size
                  .push(0) // offset
                  .push(0) // dest offset
                  .op(OpCode.CALLDATACOPY)
                  .push(0) // offset
                  .op(OpCode.MLOAD)
                  .push(100000) // gas
                  .op(OpCode.DELEGATECALL)
                  .compile())
          .build();

  final ToyAccount storingNumber =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .nonce(116)
          .address(Address.wrap(leftPadTo(Bytes.minimalBytes(1111111), 20)))
          .code(
              BytecodeCompiler.newProgram(chainConfig)
                  .op(OpCode.NUMBER) // value
                  .push(Bytes.fromHexString("c7e0")) // key
                  .op(OpCode.SSTORE)
                  .compile())
          .build();

  final ToyAccount logging =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .nonce(116)
          .address(Address.wrap(leftPadTo(Bytes.minimalBytes(222222), 20)))
          .code(
              BytecodeCompiler.newProgram(chainConfig)
                  .push(2)
                  .op(OpCode.NUMBER)
                  .op(OpCode.SUB)
                  .push(Bytes.fromHexString("c7e0")) // key
                  .op(OpCode.SLOAD)
                  .op(OpCode.SUB)
                  // We now have on the stack one arg, (NUMBER -2 ) - SLOAD
                  .push(18) // counter to go (and there STOP) if condition is true, as we want
                  // to log if false
                  .op(OpCode.JUMPI)
                  // LOG to check the output
                  .push(0)
                  .push(0)
                  .op(OpCode.LOG0)
                  .op(OpCode.STOP)
                  .op(OpCode.JUMPDEST)
                  .op(OpCode.STOP)
                  .compile())
          .build();

  final Transaction storing =
      ToyTransaction.builder()
          .sender(senderAccount)
          .to(receivingAccount)
          .payload(Bytes32.leftPad(storingNumber.getAddress()))
          .keyPair(senderKeyPair)
          .value(Wei.of(123))
          .build();

  final Transaction reading =
      ToyTransaction.builder()
          .sender(senderAccount)
          .to(receivingAccount)
          .payload(Bytes32.leftPad(logging.getAddress()))
          .keyPair(senderKeyPair)
          .value(Wei.of(123))
          .nonce(senderAccount.getNonce() + 1)
          .build();

  /**
   * Mix of empty and non empty blocks
   *
   * <ul>
   *   <li><b>E</b> stands for <b>E</b>mpty block
   *   <li><b>N</b> stands for <b>N</b>onempty block
   * </ul>
   *
   * @param runWithBesu we either run the empty block tests with a MultiBlockExecutionEnvironment or
   *     with a Besu node. Runnning them with a Besu node verifies the behavior of a conflation
   *     trace generated through RPC containing empty blocks. It can't however operate check on the
   *     tracer per say, like retrieve the hub. In the case where we run with a Besu node, we don't
   *     perform check on the hub values.
   * @param testInfo
   */
  @ParameterizedTest
  @ValueSource(booleans = {true, false})
  void mixOfEmptyAndNonEmptyBlocks_EENENE(boolean runWithBesu, TestInfo testInfo) {
    // Empty block are allowed only after Cancun
    if (isPostCancun(fork)) {

      MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder builder =
          builderFromBlockTypeList(
              List.of(
                  BlockType.EMPTY_BLOCK,
                  BlockType.EMPTY_BLOCK,
                  BlockType.MONO_TRANSACTION_BLOCK___STORING,
                  BlockType.EMPTY_BLOCK,
                  BlockType.MONO_TRANSACTION_BLOCK___READING,
                  BlockType.EMPTY_BLOCK),
              testInfo);

      final MultiBlockExecutionEnvironment env = builder.runWithBesuNode(runWithBesu).build();
      env.run();

      if (!runWithBesu) {
        final State hub = env.getHub().state();
        int nbOfLog = 0;
        for (State.HubTransactionState state : hub.getState().getAll()) {
          for (TraceSection section : state.traceSections().trace()) {
            if (section instanceof LogSection) {
              nbOfLog += 1;
            }
          }
        }
        /**
         * The idea is in one block to SSTORE the blockNumber, and few block after to SLOAD, compare
         * it with the actual block number, and if we have a match, then do a logging we could
         * check. The idea is to ensue that empty blocks are handled well, ie that the number of the
         * block number, known by besu and the tracer is updating how we assume it
         */
        checkArgument(nbOfLog == 1, "There should be exactly one log section");
      }
    }
  }

  @ParameterizedTest
  @ValueSource(booleans = {true, false})
  void mixOfEmptyAndNonEmptyBlocks_NEEN(boolean runWithBesu, TestInfo testInfo) {
    // Empty block are allowed only after Cancun
    if (isPostCancun(fork)) {

      MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder builder =
          builderFromBlockTypeList(
              List.of(
                  BlockType.MONO_TRANSACTION_BLOCK___STORING,
                  BlockType.EMPTY_BLOCK,
                  BlockType.EMPTY_BLOCK,
                  BlockType.MONO_TRANSACTION_BLOCK___READING),
              testInfo);

      final MultiBlockExecutionEnvironment env = builder.runWithBesuNode(runWithBesu).build();
      env.run();
    }
  }

  @ParameterizedTest
  @ValueSource(booleans = {true, false})
  void mixOfEmptyAndNonEmptyBlocks_NEEE(boolean runWithBesu, TestInfo testInfo) {
    // Empty block are allowed only after Cancun
    if (isPostCancun(fork)) {

      MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder builder =
          builderFromBlockTypeList(
              List.of(
                  BlockType.MONO_TRANSACTION_BLOCK___STORING,
                  BlockType.EMPTY_BLOCK,
                  BlockType.EMPTY_BLOCK,
                  BlockType.EMPTY_BLOCK),
              testInfo);

      final MultiBlockExecutionEnvironment env = builder.runWithBesuNode(runWithBesu).build();
      env.run();
    }
  }

  @ParameterizedTest
  @ValueSource(booleans = {true, false})
  void mixOfEmptyAndNonEmptyBlocks_ENNE(boolean runWithBesu, TestInfo testInfo) {
    // Empty block are allowed only after Cancun
    if (isPostCancun(fork)) {

      MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder builder =
          builderFromBlockTypeList(
              List.of(
                  BlockType.EMPTY_BLOCK,
                  BlockType.MONO_TRANSACTION_BLOCK___STORING,
                  BlockType.MONO_TRANSACTION_BLOCK___READING,
                  BlockType.EMPTY_BLOCK),
              testInfo);

      final MultiBlockExecutionEnvironment env = builder.runWithBesuNode(runWithBesu).build();
      env.run();
    }
  }

  @ParameterizedTest
  @ValueSource(booleans = {true, false})
  void mixOfEmptyAndNonEmptyBlocks_EEEE(boolean runWithBesu, TestInfo testInfo) {
    // Empty block are allowed only after Cancun
    if (isPostCancun(fork)) {

      MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder builder =
          builderFromBlockTypeList(
              List.of(
                  BlockType.EMPTY_BLOCK,
                  BlockType.EMPTY_BLOCK,
                  BlockType.EMPTY_BLOCK,
                  BlockType.EMPTY_BLOCK),
              testInfo);

      final MultiBlockExecutionEnvironment env = builder.runWithBesuNode(runWithBesu).build();
      env.run();
    }
  }

  @ParameterizedTest
  @ValueSource(booleans = {true, false})
  void mixOfEmptyAndNonEmptyBlocks_EEEN(boolean runWithBesu, TestInfo testInfo) {
    // Empty block are allowed only after Cancun
    if (isPostCancun(fork)) {

      MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder builder =
          builderFromBlockTypeList(
              List.of(
                  BlockType.EMPTY_BLOCK,
                  BlockType.EMPTY_BLOCK,
                  BlockType.EMPTY_BLOCK,
                  BlockType.MONO_TRANSACTION_BLOCK___STORING),
              testInfo);

      final MultiBlockExecutionEnvironment env = builder.runWithBesuNode(runWithBesu).build();
      env.run();
    }
  }

  @ParameterizedTest
  @ValueSource(booleans = {true, false})
  void mixOfEmptyAndNonEmptyBlocks_E(boolean runWithBesu, TestInfo testInfo) {
    // Empty blocks are allowed only after Cancun
    if (isPostCancun(fork)) {

      MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder builder =
          builderFromBlockTypeList(List.of(BlockType.EMPTY_BLOCK), testInfo);

      final MultiBlockExecutionEnvironment env = builder.runWithBesuNode(runWithBesu).build();
      env.run();
      if (!runWithBesu) {
        checkArgument(
            env.getHub().txStack().transactions().isEmpty(),
            "There should be no transaction in the state");
      }
    }
  }

  private MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder
      builderFromBlockTypeList(List<BlockType> blockTypes, TestInfo testInfo) {

    final MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder builder =
        MultiBlockExecutionEnvironment.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, storingNumber, logging, receivingAccount));

    for (BlockType blockType : blockTypes) {
      switch (blockType) {
        case EMPTY_BLOCK -> builder.addBlock(List.of());
        case MONO_TRANSACTION_BLOCK___STORING -> builder.addBlock(List.of(storing));
        case MONO_TRANSACTION_BLOCK___READING -> builder.addBlock(List.of(reading));
      }
    }

    return builder;
  }
}
