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

package net.consensys.linea.zktracer.delegation;

import static io.opentelemetry.api.internal.Utils.checkArgument;
import static net.consensys.linea.zktracer.module.hub.AccountSnapshot.EIP_7702_DELEGATION_INDICATOR_BYTES;
import static net.consensys.linea.zktracer.types.Bytecode.isDelegated;

import java.util.List;
import net.consensys.linea.UnitTestWatcher;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeCompiler;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.api.extension.ExtendWith;

@ExtendWith(UnitTestWatcher.class)
public class DelegationAndRequiresEvmTests extends TracerTestBase {

  @Test
  void delegationAndRequiresEvmTests(TestInfo testInfo) {
    final KeyPair keyPair = new SECP256K1().generateKeyPair();
    final Address senderAddress =
        Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount senderAccount =
        ToyAccount.builder().balance(Wei.fromEth(1789)).nonce(0).address(senderAddress).build();

    final KeyPair recipientKeyPair = new SECP256K1().generateKeyPair();
    final Address recipientAddress =
        Address.extract(Hash.hash(recipientKeyPair.getPublicKey().getEncodedBytes()));
    final ToyAccount recipientAccount =
        ToyAccount.builder().balance(Wei.fromEth(1)).nonce(0).address(recipientAddress).build();

    final ToyAccount eoaAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(0)
            .address(Address.fromHexString("0x0e0ae0ae0ae0ae0ae0ae0ae0ae0ae0ae0ae0ae0a"))
            .build();

    final ToyAccount smcAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(0)
            .address(Address.fromHexString("0x1111111111111111111111111111111111111111"))
            // random code, we don't care what's happening
            .code(
                BytecodeCompiler.newProgram(chainConfig)
                    .op(OpCode.PUSH0)
                    .push(2)
                    .op(OpCode.MUL)
                    .op(OpCode.PUSH0)
                    .op(OpCode.TSTORE)
                    .compile())
            .build();

    final ToyAccount delegatedAccount =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(0)
            .address(Address.fromHexString("0xde7e6a7ed0de7e6a7ed0de7e6a7ed0de7e6a7ed0"))
            .code(Bytes.concatenate(EIP_7702_DELEGATION_INDICATOR_BYTES, smcAccount.getAddress()))
            .build();

    checkArgument(
        isDelegated(delegatedAccount.getCode()), "This account is supposed to be delegated");

    // tx1, no evm (not delegated eoa) to evm (delegated to a smc)
    final Transaction tx1 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .to(recipientAccount)
            .keyPair(keyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .addCodeDelegation(chainConfig.id, smcAccount.getAddress(), 0, recipientKeyPair)
            .build();

    // tx2, evm (delegated to smc) to no evm (delegated eoa)
    final Transaction tx2 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .nonce(1L)
            .to(recipientAccount)
            .keyPair(keyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .addCodeDelegation(chainConfig.id, eoaAccount.getAddress(), 1, recipientKeyPair)
            .build();

    // tx3, no evm (delegated eoa) to evm (delegated to smc)
    final Transaction tx3 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .nonce(2L)
            .to(recipientAccount)
            .keyPair(keyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .addCodeDelegation(chainConfig.id, smcAccount.getAddress(), 2, recipientKeyPair)
            .build();

    // tx4, evm (delegated to smc) to no evm (delegation reset)
    final Transaction tx4 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .nonce(3L)
            .to(recipientAccount)
            .keyPair(keyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .addCodeDelegation(chainConfig.id, Address.ZERO, 3, recipientKeyPair)
            .build();

    // tx5: set to evm by delegation to smc
    final Transaction tx5 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .nonce(4L)
            .to(recipientAccount)
            .keyPair(keyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .addCodeDelegation(chainConfig.id, smcAccount.getAddress(), 4, recipientKeyPair)
            .build();

    // tx6: evm (delegated to smc) to evm (delegation to delegated eoa)
    final Transaction tx6 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .nonce(5L)
            .to(recipientAccount)
            .keyPair(keyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .addCodeDelegation(chainConfig.id, delegatedAccount.getAddress(), 5, recipientKeyPair)
            .build();

    // tx7: evm (delegation to delegated) to no evm (delegation to delegated eoa) passing by
    // resetting delegation, and delegate to smc
    final Transaction tx7 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .nonce(6L)
            .to(recipientAccount)
            .keyPair(keyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .addCodeDelegation(chainConfig.id, Address.ZERO, 6, recipientKeyPair)
            .addCodeDelegation(chainConfig.id, smcAccount.getAddress(), 7, recipientKeyPair)
            .addCodeDelegation(chainConfig.id, eoaAccount.getAddress(), 8, recipientKeyPair)
            .build();

    // tx8: no evm (delegation to delegated eoa) to evm (delegated to smc)
    final Transaction tx8 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .nonce(7L)
            .to(recipientAccount)
            .keyPair(keyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .addCodeDelegation(chainConfig.id, smcAccount.getAddress(), 9, recipientKeyPair)
            .build();

    // tx9: evm (delegated to smc) to evm (delegated to smc) passing by resetting delegation
    final Transaction tx9 =
        ToyTransaction.builder()
            .sender(senderAccount)
            .nonce(8L)
            .to(recipientAccount)
            .keyPair(keyPair)
            .value(Wei.of(123))
            .gasLimit(1000000L)
            .transactionType(TransactionType.DELEGATE_CODE)
            .addCodeDelegation(chainConfig.id, Address.ZERO, 10, recipientKeyPair)
            .addCodeDelegation(chainConfig.id, smcAccount.getAddress(), 11, recipientKeyPair)
            .build();

    ToyExecutionEnvironmentV2 environment =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(
                List.of(senderAccount, recipientAccount, eoaAccount, delegatedAccount, smcAccount))
            .transactions(List.of(tx1, tx2, tx3, tx4, tx5, tx6, tx7, tx8, tx9))
            .zkTracerValidator(zkTracer -> {})
            .build();

    environment.run();

    final List<TransactionProcessingMetadata> processedTxs =
        environment.getZkTracer().getHub().txStack().transactions().getAll();

    checkArgument(processedTxs.size() == 9, "Expected 9 transactions");
    checkArgument(processedTxs.get(0).requiresEvmExecution(), "Tx 1 requires evm execution");
    checkArgument(!processedTxs.get(1).requiresEvmExecution(), "Tx 2 requires no evm execution");
    checkArgument(processedTxs.get(2).requiresEvmExecution(), "Tx 3 requires evm execution");
    checkArgument(!processedTxs.get(3).requiresEvmExecution(), "Tx 4 requires no evm execution");
    checkArgument(processedTxs.get(4).requiresEvmExecution(), "Tx 5 requires evm execution");
    checkArgument(processedTxs.get(5).requiresEvmExecution(), "Tx 6 requires evm execution");
    checkArgument(!processedTxs.get(6).requiresEvmExecution(), "Tx 7 requires no evm execution");
    checkArgument(processedTxs.get(7).requiresEvmExecution(), "Tx 8 requires evm execution");
    checkArgument(processedTxs.get(8).requiresEvmExecution(), "Tx 9 requires evm execution");
  }
}
