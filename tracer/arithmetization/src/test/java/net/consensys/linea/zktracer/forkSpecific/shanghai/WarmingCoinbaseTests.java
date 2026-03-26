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

package net.consensys.linea.zktracer.forkSpecific.shanghai;

import static net.consensys.linea.testing.ToyExecutionEnvironmentV2.DEFAULT_COINBASE_ADDRESS;
import static net.consensys.linea.zktracer.module.rlpaddr.RlpAddr.CREATE2_SHIFT;
import static org.hyperledger.besu.crypto.Hash.keccak256;

import java.util.List;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;

public class WarmingCoinbaseTests extends TracerTestBase {

  @Test
  void coinbaseIsPrecompile(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(0xffff)).nonce(128).address(senderAddress).build();

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(0x1))
            .address(Address.fromHexString("0x1122334455667788990011223344556677889900"))
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipientAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(100000L)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, recipientAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .coinbase(Address.ID)
        .build()
        .run();
  }

  @Test
  void coinbaseIsSmcAndIsCalledDuringExecution(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(0xffff)).nonce(56).address(senderAddress).build();

    final ToyAccount smcCoinbase =
        ToyAccount.builder()
            .balance(Wei.fromEth(0x1))
            .address(DEFAULT_COINBASE_ADDRESS)
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    .push(1)
                    .push(2)
                    .op(OpCode.SSTORE)
                    .compile())
            .build();

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(0x1))
            .address(Address.fromHexString("0x1122334455667788990011223344556677889900"))
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    .push(0) // ret size
                    .push(0) // ret offset
                    .push(0) // arg size
                    .push(0) // arg offset
                    .push(0) // value
                    .push(DEFAULT_COINBASE_ADDRESS.getBytes()) // address
                    .push(10000) // gas
                    .op(OpCode.CALL)
                    .compile())
            .build();

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipientAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(100000L)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, recipientAccount, smcCoinbase))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .build()
        .run();
  }

  // This useless init code creates an account with bytecode "BALANCE"
  private final Bytes INIT_CODE = Bytes.fromHexString("0x603160005360016000F3");

  // = BytecodeCompiler.newProgram(chainConfig)
  //      .push(OpCode.BALANCE.byteValue())
  //      .push(0) // offset
  //      .op(OpCode.MSTORE8)
  //      .push(1) // size
  //      .push(0) // offset
  //      .op(OpCode.RETURN)
  //      .compile();

  @Test
  void coinbaseIsDeployedAddress(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(0xffff)).nonce(128).address(senderAddress).build();

    final Address deployedAddress =
        Address.contractAddress(senderAddress, senderAccount.getNonce());

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(100000L)
            .payload(INIT_CODE)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .coinbase(deployedAddress)
        .build()
        .run();
  }

  @Test
  void coinbaseIsDeployedByCreate(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(0xffff)).nonce(128).address(senderAddress).build();

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(0x12))
            .address(Address.fromHexString("0x1122334455667788990011223344556677889900"))
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    .push(INIT_CODE) // value
                    .push(0) // offset
                    .op(OpCode.MSTORE)
                    .push(INIT_CODE.size()) // size
                    .push(0) // offset
                    .push(0) // value
                    .op(OpCode.CREATE)
                    .compile())
            .build();

    final Address deployedAddress =
        Address.contractAddress(recipientAccount.getAddress(), recipientAccount.getNonce());

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(100000L)
            .to(recipientAccount)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, recipientAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .coinbase(deployedAddress)
        .build()
        .run();
  }

  @Test
  void coinbaseIsDeployedByCreate2(TestInfo testInfo) {
    final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(0xffff)).nonce(128).address(senderAddress).build();

    final Bytes32 SALT = Bytes32.ZERO;

    final ToyAccount recipientAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(0x12))
            .address(Address.fromHexString("0x1122334455667788990011223344556677889900"))
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    .push(INIT_CODE) // value
                    .push(0) // offset
                    .op(OpCode.MSTORE)
                    .push(SALT) // salt
                    .push(INIT_CODE.size()) // size
                    .push(0) // offset
                    .push(0) // value
                    .op(OpCode.CREATE2)
                    .compile())
            .build();

    final Bytes32 initCodeHash = keccak256(INIT_CODE);
    final Bytes32 hash =
        keccak256(
            Bytes.concatenate(
                CREATE2_SHIFT, recipientAccount.getAddress().getBytes(), SALT, initCodeHash));
    final Address deployedAddress = Address.extract(hash);

    final Transaction tx =
        ToyTransaction.builder()
            .sender(senderAccount)
            .keyPair(senderKeyPair)
            .value(Wei.of(123))
            .gasLimit(100000L)
            .to(recipientAccount)
            .build();

    ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
        .accounts(List.of(senderAccount, recipientAccount))
        .transaction(tx)
        .zkTracerValidator(zkTracer -> {})
        .coinbase(deployedAddress)
        .build()
        .run();
  }
}
