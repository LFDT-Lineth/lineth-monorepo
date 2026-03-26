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

package net.consensys.linea.zktracer.forkSpecific.systemTransaction;

import static com.google.common.base.Preconditions.checkArgument;
import static net.consensys.linea.testing.MultiBlockExecutionEnvironment.DEFAULT_DELTA_TIMESTAMP_BETWEEN_BLOCKS;
import static net.consensys.linea.testing.ToyExecutionEnvironmentV2.DEFAULT_BLOCK_NUMBER;
import static net.consensys.linea.testing.ToyExecutionEnvironmentV2.DEFAULT_TIME_STAMP;
import static net.consensys.linea.zktracer.forkSpecific.systemTransaction.DeployerScenario.*;
import static net.consensys.linea.zktracer.forkSpecific.systemTransaction.SystemSmartContractScenario.EXISTS_PRIOR_TO_CONFLATION;
import static net.consensys.linea.zktracer.forkSpecific.systemTransaction.SystemTransactionTestUtils.byteCodeCallingSystemSmartContract;
import static net.consensys.linea.zktracer.module.hub.fragment.transaction.system.SystemTransactionType.SYSI_EIP_2935_HISTORICAL_HASH;
import static net.consensys.linea.zktracer.module.hub.fragment.transaction.system.SystemTransactionType.SYSI_EIP_4788_BEACON_BLOCK_ROOT;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP2935HistoricalHash.EIP2935_HISTORY_STORAGE_ADDRESS;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP4788BeaconBlockRootSection.EIP4788_BEACONROOT_ADDRESS;
import static net.consensys.linea.zktracer.types.Utils.leftPadTo;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.BytecodeRunner;
import net.consensys.linea.testing.MultiBlockExecutionEnvironment;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.zktracer.module.hub.fragment.transaction.system.SystemTransactionType;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class SystemTransactionTests extends TracerTestBase {

  // random funder account
  private static final KeyPair funderOfDeployerOf2935KeyPair = new SECP256K1().generateKeyPair();
  private static final Address funderOfDeployerOf2935Address =
      Address.extract(funderOfDeployerOf2935KeyPair.getPublicKey());

  // random funder account
  private static final KeyPair funderOfDeployerOf4788KeyPair = new SECP256K1().generateKeyPair();
  private static final Address funderOfDeployerOf4788Address =
      Address.extract(funderOfDeployerOf4788KeyPair.getPublicKey());

  private final Address deployerOf2935address =
      Address.fromHexString("0x3462413Af4609098e1E27A490f554f260213D685");
  private final Address deployerOf4788address =
      Address.fromHexString("0x0B799C86a49DEeb90402691F1041aa3AF2d3C875");

  // Note: these computations are USELESS as the synthetic transactions prescribe a gas limit. We
  // can't set it ourselves in the same transaction.
  //
  // EIP-4788 init code:
  // 0x60618060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604d57602036146024575f5ffd5b5f35801560495762001fff810690815414603c575f5ffd5b62001fff01545f5260205ff35b5f5ffd5b62001fff42064281555f359062001fff015500
  //
  // weighted_byte_count = zero_bytes + 4 nonzero_bytes = 5 + 4 * 101 = 409
  // upfront cost: 21_000 + 32_000 + 4 * weighted_byte_count = 53_000 + 4 * 409 = 54_636
  // floor   cost: 21_000 + 10 * weighted_byte_count = 21_000 + 10 * 409 = 25_090
  //
  // EIP-2935 init code:
  // 0x60538060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f35600143038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500
  //
  // weighted_byte_count = zero_bytes + 4 nonzero_bytes = 1 + 4 * 91 = 365
  // upfront cost: 21_000 + 32_000 + 4 * weighted_byte_count = 53000 + 4 * 365 = 54_460
  // floor   cost: 21_000 + 10 * weighted_byte_count = 21_000 + 10 * 365 = 24_650

  final BigInteger gasPrice = new BigInteger("e8d4a51000", 16);

  // This test checks the consistency of system account by calling the system account eip-2935 in
  // happy path: not genesis block, system smart contract has code
  @Test
  void systemTransaction2935ConsistencyTest(TestInfo testInfo) {
    BytecodeRunner.of(
            byteCodeCallingSystemSmartContract(
                chainConfig, EIP2935_HISTORY_STORAGE_ADDRESS, DEFAULT_BLOCK_NUMBER - 1))
        .run(chainConfig, testInfo);
  }

  // This test checks the consistency of system account by calling the system account of eip-4788 in
  // happy path: not genesis block, system smart contract has code
  @Test
  void systemTransaction4788ConsistencyTest(TestInfo testInfo) {
    BytecodeRunner.of(
            byteCodeCallingSystemSmartContract(
                chainConfig, EIP4788_BEACONROOT_ADDRESS, DEFAULT_TIME_STAMP))
        .run(chainConfig, testInfo);
  }

  private static Stream<Arguments> scenariosForSystemContract() {

    final List<Arguments> scenarios = new ArrayList<>();
    scenarios.add(
        Arguments.of(
            EXISTS_PRIOR_TO_CONFLATION,
            PREFUNDED,
            false,
            EXISTS_PRIOR_TO_CONFLATION,
            PREFUNDED,
            false));
    for (SystemSmartContractScenario scenario2935 : SystemSmartContractScenario.values()) {
      if (scenario2935 == EXISTS_PRIOR_TO_CONFLATION) {
        continue; // already added
      }
      for (DeployerScenario deployerScenario2935 : DeployerScenario.values()) {
        for (boolean transferValueTo2935priorToDeployment : new boolean[] {false, true}) {
          for (SystemSmartContractScenario scenario4788 : SystemSmartContractScenario.values()) {
            if (scenario4788 == EXISTS_PRIOR_TO_CONFLATION) {
              continue; // already added
            }
            for (DeployerScenario deployerScenario4788 : DeployerScenario.values()) {
              for (boolean transferValueTo4788priorToDeployment : new boolean[] {false, true}) {
                scenarios.add(
                    Arguments.of(
                        scenario2935,
                        deployerScenario2935,
                        transferValueTo2935priorToDeployment,
                        scenario4788,
                        deployerScenario4788,
                        transferValueTo4788priorToDeployment));
              }
            }
          }
        }
      }
    }
    return scenarios.stream();
  }

  static final long senderNonce = 0L;

  // random sender account that will send a tx to system account
  private static final KeyPair senderKeyPair = new SECP256K1().generateKeyPair();
  private static final Address senderAddress = Address.extract(senderKeyPair.getPublicKey());

  // f1d7 ⇔ f(un)d(er)
  private static final ToyAccount funderOf2935deployer =
      ToyAccount.builder()
          .balance(Wei.fromEth(4))
          .nonce(69)
          .address(funderOfDeployerOf2935Address)
          .build();

  private static final ToyAccount funderOf4788deployer =
      ToyAccount.builder()
          .balance(Wei.fromEth(5))
          .nonce(420)
          .address(funderOfDeployerOf4788Address)
          .build();

  private static final ToyAccount senderAccount =
      ToyAccount.builder()
          .balance(Wei.fromEth(123))
          .nonce(senderNonce)
          .address(senderAddress)
          .balance(Wei.fromEth(12))
          .build();

  // This EOA calls 3 times the system account with as input the three block number of the
  // conflation
  private static final ToyAccount callerOf2935 =
      ToyAccount.builder()
          .address(Address.wrap(leftPadTo(Bytes.fromHexString("0x2935"), Address.SIZE)))
          .balance(Wei.fromEth(1))
          .code(
              Bytes.concatenate(
                  byteCodeCallingSystemSmartContract(
                      chainConfig, EIP2935_HISTORY_STORAGE_ADDRESS, 0),
                  byteCodeCallingSystemSmartContract(
                      chainConfig, EIP2935_HISTORY_STORAGE_ADDRESS, 1),
                  byteCodeCallingSystemSmartContract(
                      chainConfig, EIP2935_HISTORY_STORAGE_ADDRESS, 2)))
          .build();

  // This EOA calls 3 times the system account with as input the three block timestamp of the
  // conflation
  private static final ToyAccount callerOf4788 =
      ToyAccount.builder()
          .address(Address.wrap(leftPadTo(Bytes.fromHexString("0x4788"), Address.SIZE)))
          .balance(Wei.fromEth(1))
          .code(
              Bytes.concatenate(
                  byteCodeCallingSystemSmartContract(
                      chainConfig, EIP4788_BEACONROOT_ADDRESS, DEFAULT_TIME_STAMP),
                  byteCodeCallingSystemSmartContract(
                      chainConfig,
                      EIP4788_BEACONROOT_ADDRESS,
                      DEFAULT_TIME_STAMP + DEFAULT_DELTA_TIMESTAMP_BETWEEN_BLOCKS),
                  byteCodeCallingSystemSmartContract(
                      chainConfig,
                      EIP4788_BEACONROOT_ADDRESS,
                      DEFAULT_TIME_STAMP + 2 * DEFAULT_DELTA_TIMESTAMP_BETWEEN_BLOCKS)))
          .build();

  @ParameterizedTest
  @MethodSource("scenariosForSystemContract")
  void genesisBlockTest(
      SystemSmartContractScenario scenario2935,
      DeployerScenario deployerScenario2935,
      boolean transferValueTo2935priorToDeployment,
      SystemSmartContractScenario scenario4788,
      DeployerScenario deployerScenario4788,
      boolean transferValueTo4788priorToDeployment,
      TestInfo testInfo) {

    // Note: do not modify me: value coming from https://eips.ethereum.org/EIPS/eip-2935
    final ToyAccount deployerOf2935 =
        ToyAccount.builder()
            .nonce(0)
            .address(deployerOf2935address)
            .balance(initialDeployerBalance(SYSI_EIP_2935_HISTORICAL_HASH, deployerScenario2935))
            .build();

    final Transaction deploy2935 =
        ToyTransaction.builder()
            .sender(deployerOf2935)
            .transactionType(TransactionType.FRONTIER)
            .gasLimit(0x3d090L)
            .gasPrice(Wei.of(gasPrice))
            .payload(
                Bytes.fromHexString(
                    "0x60538060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604657602036036042575f35600143038111604257611fff81430311604257611fff9006545f5260205ff35b5f5ffd5b5f35611fff60014303065500"))
            .value(Wei.of(0))
            .signature(
                Bytes.concatenate(
                    Bytes32.fromHexStringLenient("0x539"), // r
                    Bytes32.fromHexStringLenient("0xaa12693182426612186309f02cfe8a80a0000"), // s
                    Bytes.fromHexString("0x00") // v = 0x1B
                    ))
            .build();

    // Note: do not modify me: value coming from https://eips.ethereum.org/EIPS/eip-4788
    final ToyAccount deployerOf4788 =
        ToyAccount.builder()
            .nonce(0)
            .address(deployerOf4788address)
            .balance(initialDeployerBalance(SYSI_EIP_4788_BEACON_BLOCK_ROOT, deployerScenario4788))
            .build();

    final Transaction deploy4788 =
        ToyTransaction.builder()
            .sender(deployerOf4788)
            .transactionType(TransactionType.FRONTIER)
            .gasLimit(0x3d090L)
            .gasPrice(Wei.of(gasPrice))
            .payload(
                Bytes.fromHexString(
                    "0x60618060095f395ff33373fffffffffffffffffffffffffffffffffffffffe14604d57602036146024575f5ffd5b5f35801560495762001fff810690815414603c575f5ffd5b62001fff01545f5260205ff35b5f5ffd5b62001fff42064281555f359062001fff015500"))
            .value(Wei.of(0))
            .signature(
                Bytes.concatenate(
                    Bytes32.fromHexStringLenient("0x539"), // r
                    Bytes32.fromHexStringLenient("0x1b9b6eb1f0"), // s
                    Bytes.fromHexString("0x00") // v = 0x1B
                    ))
            .build();

    final List<Transaction> transactionsInGenesisBlock = new ArrayList<>();
    long nonce = 0;

    transactionsInGenesisBlock.add(check2935Tx(nonce++));
    transactionsInGenesisBlock.add(check4788Tx(nonce++));

    if (transferValueTo2935priorToDeployment) {
      transactionsInGenesisBlock.add(transferValueTo2935Tx(nonce++));
    }
    if (transferValueTo4788priorToDeployment) {
      transactionsInGenesisBlock.add(transferValueTo4788Tx(nonce++));
    }

    if (!deployerScenario2935.isPrefunded()) {
      transactionsInGenesisBlock.add(fund2935deployerTx(deployerScenario2935));
    }
    if (!deployerScenario4788.isPrefunded()) {
      transactionsInGenesisBlock.add(fund4788deployerTx(deployerScenario4788));
    }

    if (scenario2935 == SystemSmartContractScenario.DEPLOY_IN_GENESIS_BLOCK) {
      transactionsInGenesisBlock.add(deploy2935);
    }
    if (scenario4788 == SystemSmartContractScenario.DEPLOY_IN_GENESIS_BLOCK) {
      transactionsInGenesisBlock.add(deploy4788);
    }
    transactionsInGenesisBlock.add(check2935Tx(nonce++));
    transactionsInGenesisBlock.add(check4788Tx(nonce++));

    final List<Transaction> transactionsInBlock1 = new ArrayList<>();
    transactionsInBlock1.add(check2935Tx(nonce++));
    transactionsInBlock1.add(check4788Tx(nonce++));

    if (scenario2935 == SystemSmartContractScenario.DEPLOY_IN_FIRST_BLOCK) {
      transactionsInBlock1.add(deploy2935);
      transactionsInBlock1.add(check2935Tx(nonce++));
    }
    if (scenario4788 == SystemSmartContractScenario.DEPLOY_IN_FIRST_BLOCK) {
      transactionsInBlock1.add(deploy4788);
      transactionsInBlock1.add(check4788Tx(nonce++));
    }

    final List<Transaction> transactionsInBlock2 = new ArrayList<>();
    transactionsInBlock2.add(check2935Tx(nonce++));
    transactionsInBlock2.add(check4788Tx(nonce++));

    final MultiBlockExecutionEnvironment.MultiBlockExecutionEnvironmentBuilder builder =
        MultiBlockExecutionEnvironment.builder(
            chainConfig,
            testInfo,
            scenario2935 == EXISTS_PRIOR_TO_CONFLATION
                || scenario4788 == EXISTS_PRIOR_TO_CONFLATION,
            0);
    builder
        .accounts(
            List.of(
                senderAccount,
                deployerOf2935,
                deployerOf4788,
                callerOf2935,
                callerOf4788,
                funderOf2935deployer,
                funderOf4788deployer))
        .addBlock(transactionsInGenesisBlock)
        .addBlock(transactionsInBlock1)
        .addBlock(transactionsInBlock2)
        .build()
        .run();
  }

  private Transaction check2935Tx(long nonce) {
    return ToyTransaction.builder()
        .sender(senderAccount)
        .to(callerOf2935)
        .nonce(nonce)
        .keyPair(senderKeyPair)
        .build();
  }

  private Transaction check4788Tx(long nonce) {
    return ToyTransaction.builder()
        .sender(senderAccount)
        .to(callerOf4788)
        .nonce(nonce)
        .keyPair(senderKeyPair)
        .build();
  }

  private Transaction transferValueTo2935Tx(long nonce) {
    return ToyTransaction.builder()
        .sender(senderAccount)
        .toAddress(EIP2935_HISTORY_STORAGE_ADDRESS)
        .nonce(nonce)
        .value(Wei.ONE)
        .keyPair(senderKeyPair)
        .build();
  }

  private Transaction transferValueTo4788Tx(long nonce) {
    return ToyTransaction.builder()
        .sender(senderAccount)
        .toAddress(EIP4788_BEACONROOT_ADDRESS)
        .nonce(nonce)
        .value(Wei.ONE)
        .keyPair(senderKeyPair)
        .build();
  }

  private Transaction fund2935deployerTx(DeployerScenario scenario) {
    checkArgument(!scenario.isPrefunded());

    return ToyTransaction.builder()
        .sender(funderOf2935deployer)
        .toAddress(deployerOf2935address)
        .nonce(funderOf2935deployer.getNonce())
        .value(fundingAmount(SYSI_EIP_2935_HISTORICAL_HASH, scenario))
        .keyPair(funderOfDeployerOf2935KeyPair)
        .build();
  }

  private Transaction fund4788deployerTx(DeployerScenario scenario) {
    checkArgument(!scenario.isPrefunded());

    return ToyTransaction.builder()
        .sender(funderOf4788deployer)
        .toAddress(deployerOf4788address)
        .nonce(funderOf4788deployer.getNonce())
        .value(fundingAmount(SYSI_EIP_4788_BEACON_BLOCK_ROOT, scenario))
        .keyPair(funderOfDeployerOf4788KeyPair)
        .build();
  }

  /**
   * Returns the appropriate initial balance for the deployer of the relevant {@code type} of system
   * transaction. This is 0 if the {@code scenario} is NOT {@link DeployerScenario#isPrefunded()}.
   *
   * @param scenario
   * @return
   */
  private Wei initialDeployerBalance(SystemTransactionType type, DeployerScenario scenario) {
    checkArgument(type == SYSI_EIP_4788_BEACON_BLOCK_ROOT || type == SYSI_EIP_2935_HISTORICAL_HASH);

    return switch (scenario) {
      case PREFUNDED -> Wei.fromEth(3);
      case FUNDED_IN_BLOCK -> Wei.ZERO;
    };
  }

  private Wei fundingAmount(SystemTransactionType type, DeployerScenario scenario) {
    checkArgument(type == SYSI_EIP_4788_BEACON_BLOCK_ROOT || type == SYSI_EIP_2935_HISTORICAL_HASH);
    checkArgument(!scenario.isPrefunded());

    return Wei.fromEth(2);
  }
}
