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

package net.consensys.linea.zktracer.module.mmu;

import static net.consensys.linea.testing.BytecodeCompiler.newProgram;
import static net.consensys.linea.zktracer.utilities.Utils.*;

import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
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

/**
 * This test aims at testing a mixture of un reverted, selfreverted (STATIC exception, or REVERT),
 * and get reverted LOGX in order to tests that the MMU and LOG stamps work as intended wrt
 * reverting LOGX operations
 */
public class RevertingLogsTests extends TracerTestBase {

  @Test
  void mixtureOfRevertedLogs(TestInfo testInfo) {

    final Bytes TOPIC_1 =
        Bytes.fromHexString("0x000007031c100000000007031c100000000007031c100000000007031c100000");

    final Bytes TOPIC_2 =
        Bytes.fromHexString("0x000007031c200000000007031c200000000007031c200000000007031c200000");

    final Bytes TOPIC_3 =
        Bytes.fromHexString("0x000007031c300000000007031c300000000007031c300000000007031c300000");

    final Bytes TOPIC_4 =
        Bytes.fromHexString("0x000007031c400000000007031c400000000007031c400000000007031c400000");

    final Bytes LOG0 =
        newProgram(chainConfig)
            .push(18) // size
            .push(0x1) // offset
            .op(OpCode.LOG0)
            .compile();

    final Bytes LOG1 =
        newProgram(chainConfig)
            .push(TOPIC_1)
            .push(18) // size
            .push(0x1) // offset
            .op(OpCode.LOG1)
            .compile();

    final Bytes LOG2 =
        newProgram(chainConfig)
            .push(TOPIC_2) // topic 2
            .push(TOPIC_1) // topic 1
            .push(18) // size
            .push(0x1) // offset
            .op(OpCode.LOG2)
            .compile();

    final Bytes LOG3 =
        newProgram(chainConfig)
            .push(TOPIC_3) // topic 3
            .push(TOPIC_2) // topic 2
            .push(TOPIC_1) // topic 1
            .push(18) // size
            .push(0x1) // offset
            .op(OpCode.LOG3)
            .compile();

    final Bytes LOG4 =
        newProgram(chainConfig)
            .push(TOPIC_4) // topic 4
            .push(TOPIC_3) // topic 3
            .push(TOPIC_2) // topic 2
            .push(TOPIC_1) // topic 1
            .push(18) // size
            .push(0x1) // offset
            .op(OpCode.LOG4)
            .compile();

    final Bytes SELFREVERT_LOG_BYTECODE =
        newProgram(chainConfig)
            .immediate(POPULATE_MEMORY)
            .immediate(LOG3)
            .immediate(REVERT)
            .compile();

    final Bytes NON_REVERTING_LOG_BYTECODE =
        newProgram(chainConfig).immediate(POPULATE_MEMORY).immediate(LOG4).compile();

    final ToyAccount nonRevertingLogSMC =
        ToyAccount.builder()
            .address(Address.fromHexString("0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"))
            .code(NON_REVERTING_LOG_BYTECODE)
            .balance(Wei.of(98989898))
            .build();

    final ToyAccount selfRevertingLogSMC =
        ToyAccount.builder()
            .address(Address.fromHexString("0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"))
            .code(SELFREVERT_LOG_BYTECODE)
            .balance(Wei.of(1235678))
            .build();

    final ToyAccount callNonRevertLogAndSelfRevert =
        ToyAccount.builder()
            .address(Address.fromHexString("0xcccccccccccccccccccccccccccccccccccccccc"))
            .code(Bytes.concatenate(call(nonRevertingLogSMC.getAddress(), false), REVERT))
            .balance(Wei.of(10000))
            .build();

    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.of(100000000)).address(senderAddress).build();

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.of(10000))
            .address(Address.fromHexString("0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"))
            .code(
                Bytes.concatenate(
                    POPULATE_MEMORY,
                    LOG0,
                    call(nonRevertingLogSMC.getAddress(), false),
                    call(nonRevertingLogSMC.getAddress(), true),
                    LOG1,
                    call(selfRevertingLogSMC.getAddress(), false),
                    LOG2,
                    call(callNonRevertLogAndSelfRevert.getAddress(), false),
                    LOG4))
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(keyPair)
            .gasLimit(1000000L)
            .gasPrice(Wei.of(10L))
            .payload(
                Bytes.fromHexString(
                    "0x00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"))
            .to(recipientAccount)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(
            List.of(
                senderAccount,
                selfRevertingLogSMC,
                callNonRevertLogAndSelfRevert,
                nonRevertingLogSMC,
                recipientAccount))
        .transaction(tx)
        .build()
        .run();
  }
}
