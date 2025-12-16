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

package net.consensys.linea;

import static net.consensys.linea.plugins.config.LineaL1L2BridgeSharedConfiguration.TEST_DEFAULT;
import static net.consensys.linea.zktracer.Utils.call;
import static net.consensys.linea.zktracer.Utils.delegateCall;
import static net.consensys.linea.zktracer.module.ModuleName.*;
import static org.junit.jupiter.api.Assertions.*;

import java.util.List;
import java.util.Map;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.*;
import net.consensys.linea.zktracer.ZkCounter;
import net.consensys.linea.zktracer.container.module.Module;
import net.consensys.linea.zktracer.module.ModuleName;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class ZkCounterTest extends TracerTestBase {

  @Test
  void checkedAndUncheckedAreExclusive() {
    final ZkCounter counter = new ZkCounter(chainConfig.bridgeConfiguration, fork);
    final List<Module> allModules = counter.getModulesToCount();
    final List<Module> checked = counter.checkedModules();
    final List<Module> unchecked = counter.uncheckedModules();

    for (Module m : checked) {
      assertFalse(
          unchecked.contains(m),
          "Module " + m.moduleKey() + " is in both checked and unchecked list");
      assertTrue(
          allModules.contains(m),
          "Module " + m.moduleKey() + " is in checked but not in all modules list");
    }

    for (Module m : unchecked) {
      assertFalse(
          checked.contains(m),
          "Module " + m.moduleKey() + " is in both checked and unchecked list");
      assertTrue(
          allModules.contains(m),
          "Module " + m.moduleKey() + " is in checked but not in all modules list");
    }

    for (Module m : allModules) {
      assertTrue(
          checked.contains(m) || unchecked.contains(m),
          "Module " + m.moduleKey() + " is in all modules but not in checked or unchecked list");
    }
  }

  @Test
  void twoSuccessfullL2l1Logs(TestInfo testInfo) {
    // sender account
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(123)).nonce(12).address(senderAddress).build();

    // receiver account: L2L1Logs account
    final ToyAccount l2l1LogSMC =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .address(TEST_DEFAULT.contract())
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    // LOG1 with right topic, and no data
                    .push(TEST_DEFAULT.topic()) // topic
                    .push(0) //  size
                    .push(0) // offset
                    .op(OpCode.LOG1)
                    // LOG4 with right first topic, and useless data
                    .push(2) // value
                    .push(2) // offset
                    .op(OpCode.MSTORE)
                    .push(Bytes.of(4)) // topic 4
                    .push(Bytes.of(3)) // topic 3
                    .push(Bytes.of(2)) // topic 2
                    .push(TEST_DEFAULT.topic()) // topic 1
                    .push(15) //  size
                    .push(0) // offset
                    .op(OpCode.LOG1)
                    .compile())
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(l2l1LogSMC)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .value(Wei.of(1000))
            .build();

    final ToyExecutionEnvironmentV2 toyWorld =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, l2l1LogSMC))
            .transaction(tx)
            .zkTracerValidator(zkTracer -> {})
            .build();

    toyWorld.runForCounting();

    final Map<String, Integer> lineCountMap = toyWorld.getZkCounter().getModulesLineCount();

    // We made two LOGs, one with 1 topic, and one with 4 topics and data
    assertEquals(2, lineCountMap.get(BLOCK_L2_L1_LOGS.toString()));

    // no precompile call:
    assertEquals(0, lineCountMap.get(ModuleName.PRECOMPILE_MODEXP_EFFECTIVE_CALLS.toString()));
    assertEquals(0, lineCountMap.get(PRECOMPILE_RIPEMD_BLOCKS.toString()));
    assertEquals(0, lineCountMap.get(PRECOMPILE_BLAKE_EFFECTIVE_CALLS.toString()));

    // L1 block size > 0
    assertTrue(lineCountMap.get(BLOCK_L1_SIZE.toString()) > 0);
  }

  @Test
  /**
   * This test does: - a LOG with right address and topic, but reverted - a LOG with right Address,
   * but the right topic not at the right place - a LOG with right topic, but wrong address
   */
  void unsuccessfullL2l1Logs(TestInfo testInfo) {
    // sender account
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(123)).nonce(12).address(senderAddress).build();

    final ToyAccount LOG_ACCOUNT =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .address(Address.wrap(Bytes.repeat((byte) 1, Address.SIZE)))
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    // LOG1 with right topic, and no data
                    .push(TEST_DEFAULT.topic()) // topic
                    .push(0) //  size
                    .push(0) // offset
                    .op(OpCode.LOG1)
                    .compile())
            .build();

    final ToyAccount LOG_ACCOUNT_AND_REVERT =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .address(Address.wrap(Bytes.repeat((byte) 2, Address.SIZE)))
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    // LOG1 with right topic, and no data
                    .push(TEST_DEFAULT.topic()) // topic
                    .push(0) //  size
                    .push(0) // offset
                    .op(OpCode.LOG1)
                    .push(0)
                    .push(0)
                    .op(OpCode.REVERT)
                    .compile())
            .build();

    // receiver account: L2L1Logs account
    final ToyAccount l2l1LogSMC =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .address(TEST_DEFAULT.contract())
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    // LOG2 with right topic, not at the right place
                    .push(TEST_DEFAULT.topic()) // topic 2
                    .push(Bytes.of(1)) // topic 1
                    .push(0) //  size
                    .push(0) // offset
                    .op(OpCode.LOG2)
                    // DELEGATE CALL to a contract that will LOG1 then revert
                    .immediate(delegateCall(LOG_ACCOUNT_AND_REVERT.getAddress()))
                    // LOG1 with right topic, but not right address
                    .immediate(call(LOG_ACCOUNT.getAddress(), false))
                    .compile())
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(l2l1LogSMC)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .value(Wei.of(1000))
            .build();

    final ToyExecutionEnvironmentV2 toyWorld =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(senderAccount, l2l1LogSMC, LOG_ACCOUNT_AND_REVERT, LOG_ACCOUNT))
            .transaction(tx)
            .zkTracerValidator(zkTracer -> {})
            .build();

    toyWorld.runForCounting();

    final Map<String, Integer> lineCountMap = toyWorld.getZkCounter().getModulesLineCount();

    // We made a reverted LOG, LOG with wrong address, and LOG with TOPIC at the wrong place
    assertEquals(0, lineCountMap.get(BLOCK_L2_L1_LOGS.toString()));

    // no precompile call:
    assertEquals(0, lineCountMap.get(ModuleName.PRECOMPILE_MODEXP_EFFECTIVE_CALLS.toString()));
    assertEquals(0, lineCountMap.get(PRECOMPILE_RIPEMD_BLOCKS.toString()));
    assertEquals(0, lineCountMap.get(PRECOMPILE_BLAKE_EFFECTIVE_CALLS.toString()));

    // L1 block size > 0
    assertTrue(lineCountMap.get(BLOCK_L1_SIZE.toString()) > 0);
  }
}
