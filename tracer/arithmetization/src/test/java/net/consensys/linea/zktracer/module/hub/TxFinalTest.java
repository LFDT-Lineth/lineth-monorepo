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

package net.consensys.linea.zktracer.module.hub;

import static net.consensys.linea.zktracer.types.AddressUtils.getCreateRawAddress;

import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
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

public class TxFinalTest extends TracerTestBase {

  // sender account
  private static final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
  private static final Address senderAddress =
      Address.extract(Hash.hash(senderKeyPair.getPublicKey().getEncodedBytes()));
  private static final ToyAccount senderAccount =
      ToyAccount.builder().balance(Wei.fromEth(123)).nonce(12).address(senderAddress).build();

  // receiver account: dummy SMC account
  private static final ToyAccount receiverAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(1))
          .address(Address.fromHexString("0xdead000000000000000000000000000beef"))
          .code(BytecodeCompiler.newProgram(chainConfig).push(12).push(35).op(OpCode.SGT).compile())
          .build();

  private static final Bytes initCode =
      BytecodeCompiler.newProgram(chainConfig)
          .push(12)
          .push(13)
          .push(24)
          .op(OpCode.ADDMOD)
          .compile();

  final Address depAddress =
      Address.extract(getCreateRawAddress(senderAddress, senderAccount.getNonce()));

  // TODO: add smcCallSenderIsRecipient() {}, not possible before EIP-7702

  @Test
  void smcCallSenderIsCoinbase(TestInfo testInfo) {
    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .value(Wei.of(1000))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, receiverAccount))
        .transaction(tx)
        .coinbase(senderAddress)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  @Test
  void smcCallRecipientIsCoinbase(TestInfo testInfo) {
    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(receiverAccount)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .value(Wei.of(1000))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, receiverAccount))
        .transaction(tx)
        .coinbase(receiverAccount.getAddress())
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  // TODO: add smcCallTripleCollision() {}, not possible before EIP-7702

  // good luck for finding the right nonce ;) deploymentSenderIsRecipient() {}

  @Test
  void deploymentSenderIsCoinbase(TestInfo testInfo) {
    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .payload(initCode)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .value(Wei.of(1000))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .coinbase(senderAddress)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  @Test
  void deploymentRecipientIsCoinbase(TestInfo testInfo) {
    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .payload(initCode)
            .keyPair(senderKeyPair)
            .gasLimit(300000L)
            .value(Wei.of(1000))
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .coinbase(depAddress)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }
  // good luck for finding the right nonce ;) deploymentTripleCollision() {}
}
