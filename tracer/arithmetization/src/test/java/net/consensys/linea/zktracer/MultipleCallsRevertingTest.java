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

package net.consensys.linea.zktracer;

import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.ToyTransaction;
import net.consensys.linea.testing.TransactionProcessingResultValidator;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.crypto.KeyPair;
import org.hyperledger.besu.crypto.SECP256K1;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Hash;
import org.hyperledger.besu.datatypes.TransactionType;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

public class MultipleCallsRevertingTest extends TracerTestBase {
  // See https://github.com/Consensys/linea-tracer/issues/1172 for documentation

  enum CallCase {
    BASE, // Last call send part of the funds of FundsSender to FundsReceiver1
    SEND_ALL, // Last call send all the funds of FundsSender to FundsReceiver1
    INVOKE_TIP_THE_SENDER // Last call pat of the funds of FundsSender to FundsReceiver1 and
    // FundsReceiver1 sends back half of the received amount as a tip
  }

  /**
   * Parameterized test for contract SLOAD and SSTORE operations.
   *
   * @param toRoot whether the first call is to FundsSenderRoot or to FundsSender
   * @param mustRevert whether the transaction must revert
   * @param callCase the type of the last call
   * @param useCallCode whether to use CALLCODE or CALL
   */
  @ParameterizedTest
  @MethodSource("contractForSLoadAndSStoreTestSource")
  void multipleCallsRevertingTest(
      boolean toRoot,
      boolean mustRevert,
      CallCase callCase,
      boolean useCallCode,
      TestInfo testInfo) {
    // arithmetization/src/test/resources/contracts/multipleCallsReverting/*.sol
    // solc-select use 0.4.24
    // solc *.sol --bin-runtime --evm-version byzantium -o compiledContracts
    // We need to use byzantium because as callcode is deprecated in newer versions

    Address addressFundsSenderRoot =
        Address.fromHexString("0xC3Ba5050Ec45990f76474163c5bA673c244aaECA");
    Address addressFundsSender =
        Address.fromHexString("0xE3Ca443c9fd7AF40A2B5a95d43207E763e56005F");
    Address addressFundsReceiver1 =
        Address.fromHexString("0xd7Ca4e99F7C171B9ea2De80d3363c47009afaC5F");
    Address addressFundsReceiver2 =
        Address.fromHexString("0x0813d4a158d06784FDB48323344896B2B1aa0F85");

    // User address
    KeyPair keyPair = new SECP256K1().generateKeyPair();
    Address userAddress = Address.extract(Hash.hash(keyPair.getPublicKey().getEncodedBytes()));
    ToyAccount userAccount =
        ToyAccount.builder().balance(Wei.fromEth(100)).nonce(1).address(userAddress).build();

    // FundsSenderRoot
    ToyAccount contractAccountFundsSenderRoot =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(2)
            .address(addressFundsSenderRoot)
            .code(
                Bytes.fromHexString(
                    "608060405260043610610041576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063625fd83914610046575b600080fd5b34801561005257600080fd5b506100e0600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803515159060200190929190803560ff1690602001909291905050506100e2565b005b8473ffffffffffffffffffffffffffffffffffffffff1663c74c79b5858585856040518563ffffffff167c0100000000000000000000000000000000000000000000000000000000028152600401808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001831515151581526020018260028111156101ab57fe5b60ff168152602001945050505050600060405180830381600087803b1580156101d357600080fd5b505af11580156101e7573d6000803e3d6000fd5b5050505050505050505600a165627a7a7230582009e628f4da7263c6eea8cc491bd581b01b2aa8850e8bf362d138b66d89dd795a0029"))
            .build();

    // FundsSender
    ToyAccount contractAccountFundsSender =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(3)
            .address(addressFundsSender)
            .code(
                Bytes.fromHexString(
                    "60806040526004361061004c576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680638b5cedce1461004e578063c74c79b514610065575b005b34801561005a57600080fd5b506100636100e1565b005b34801561007157600080fd5b506100df600480360381019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803515159060200190929190803560ff1690602001909291905050506100fd565b005b60016000806101000a81548160ff021916908315150217905550565b60006801236efcbcbb3400003073ffffffffffffffffffffffffffffffffffffffff16311415156101bc576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252602d8152602001807f46756e647353656e6465722062616c616e6365206d757374206265206174206c81526020017f656173742032312065746865720000000000000000000000000000000000000081525060400191505060405180910390fd5b6101ce85670de0b6b3a76400006105ee565b9050801515610245576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252600d8152602001807f63616c6c2031206661696c65640000000000000000000000000000000000000081525060200191505060405180910390fd5b61025730671bc16d674ec800006105ee565b90508015156102ce576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252600d8152602001807f63616c6c2032206661696c65640000000000000000000000000000000000000081525060200191505060405180910390fd5b6102e0856729a2241af62c00006105ee565b9050801515610357576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252600d8152602001807f63616c6c2033206661696c65640000000000000000000000000000000000000081525060200191505060405180910390fd5b61036984673782dace9d9000006105ee565b90508015156103e0576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252600d8152602001807f63616c6c2034206661696c65640000000000000000000000000000000000000081525060200191505060405180910390fd5b6103f230674563918244f400006105ee565b9050801515610469576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252600d8152602001807f63616c6c2035206661696c65640000000000000000000000000000000000000081525060200191505060405180910390fd5b6000600281111561047657fe5b82600281111561048257fe5b14156105125761049a856753444835ec5800006105ee565b9050801515610511576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252600d8152602001807f63616c6c2036206661696c65640000000000000000000000000000000000000081525060200191505060405180910390fd5b5b6001600281111561051f57fe5b82600281111561052b57fe5b141561053f5761053a8561067d565b610571565b60028081111561054b57fe5b82600281111561055757fe5b14156105705761056f856753444835ec58000061071b565b5b5b821515156105e7576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260148152602001807f726576657274207472616e7366657246756e647300000000000000000000000081525060200191505060405180910390fd5b5050505050565b6000806000809054906101000a900460ff161561063e578373ffffffffffffffffffffffffffffffffffffffff168360405180602001905060006040518083038185875af2925050509050610673565b8373ffffffffffffffffffffffffffffffffffffffff168360405180602001905060006040518083038185875af19250505090505b8091505092915050565b60006106a0823073ffffffffffffffffffffffffffffffffffffffff16316105ee565b9050801515610717576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601b8152602001807f63616c6c203620776974682073656e6420616c6c206661696c6564000000000081525060200191505060405180910390fd5b5050565b60008060009054906101000a900460ff161561085f578273ffffffffffffffffffffffffffffffffffffffff16826000809054906101000a900460ff1660405160240180821515151581526020019150506040516020818303038152906040527fe1462a6e000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff838183161783525050505060405180828051906020019080838360005b838110156108165780820151818401526020810190506107fb565b50505050905090810190601f1680156108435780820380516001836020036101000a031916815260200191505b5091505060006040518083038185875af2925050509050610989565b8273ffffffffffffffffffffffffffffffffffffffff16826000809054906101000a900460ff1660405160240180821515151581526020019150506040516020818303038152906040527fe1462a6e000000000000000000000000000000000000000000000000000000007bffffffffffffffffffffffffffffffffffffffffffffffffffffffff19166020820180517bffffffffffffffffffffffffffffffffffffffffffffffffffffffff838183161783525050505060405180828051906020019080838360005b83811015610944578082015181840152602081019050610929565b50505050905090810190601f1680156109715780820380516001836020036101000a031916815260200191505b5091505060006040518083038185875af19250505090505b801515610a24576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260288152602001807f63616c6c2035207769746820696e766f6b6520746970207468652073656e646581526020017f72206661696c656400000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b5050505600a165627a7a72305820aa10ce4d00c2af244e59aaebaf7d407e3065cc9c168c8c26d9b5d8c359c088750029"))
            .build();

    // FundsReceiver1
    ToyAccount contractAccountFundsReceiver1 =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(4)
            .address(addressFundsReceiver1)
            .code(
                Bytes.fromHexString(
                    "608060405260043610610041576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff168063e1462a6e14610043575b005b610063600480360381019080803515159060200190929190505050610065565b005b60008060023481151561007457fe5b04915082156100b6573373ffffffffffffffffffffffffffffffffffffffff168260405180602001905060006040518083038185875af29250505090506100eb565b3373ffffffffffffffffffffffffffffffffffffffff168260405180602001905060006040518083038185875af19250505090505b801515610160576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601c8152602001807f46523120636f756c64206e6f7420746970207468652073656e6465720000000081525060200191505060405180910390fd5b5050505600a165627a7a72305820e8c24a11f06361cc45d3cc927eed371de4106a71f99ace2adbd527baaae3c9710029"))
            .build();

    // FundsReceiver2
    ToyAccount contractAccountFundsReceiver2 =
        ToyAccount.builder()
            .balance(Wei.fromEth(1))
            .nonce(5)
            .address(addressFundsReceiver2)
            .code(
                Bytes.fromHexString(
                    "60806040520000a165627a7a72305820b2f48dd4fef9d7e4fc5e7f808c778f36d5a3388d0a45fcdb56556a3b9709844a0029"))
            .build();

    // Send funds to FundsSender
    Transaction txToSendFundsToFundsSender =
        ToyTransaction.builder()
            .sender(userAccount)
            .to(contractAccountFundsSender)
            .transactionType(TransactionType.FRONTIER)
            .value(Wei.fromEth(21))
            .keyPair(keyPair)
            .nonce(1L)
            .gasLimit(0xffffffL)
            .build();

    // Invoke turnOnUseCallCode in FundsSender
    Transaction txToInvokeTurnOnUseCallCodeInFundsSender =
        ToyTransaction.builder()
            .sender(userAccount)
            .to(contractAccountFundsSender)
            .payload(Bytes.fromHexString("0x8b5cedce"))
            .transactionType(TransactionType.FRONTIER)
            .value(Wei.ZERO)
            .keyPair(keyPair)
            .nonce(2L)
            .gasLimit(0xffffffL)
            .build();

    // Initiate calls
    String payload =
        toRoot
            ? invokeFundsSender(
                addressFundsSender,
                addressFundsReceiver1,
                addressFundsReceiver2,
                mustRevert,
                callCase)
            : transferFunds(addressFundsReceiver1, addressFundsReceiver2, mustRevert, callCase);

    Transaction txToInitiateCalls =
        ToyTransaction.builder()
            .sender(userAccount)
            .to(toRoot ? contractAccountFundsSenderRoot : contractAccountFundsSender)
            .payload(Bytes.fromHexString(payload))
            .transactionType(TransactionType.FRONTIER)
            .value(Wei.ZERO)
            .keyPair(keyPair)
            .nonce(useCallCode ? 3L : 2L)
            .gasLimit(0xffffffL)
            .build();

    List<ToyAccount> accounts =
        List.of(
            userAccount,
            contractAccountFundsSenderRoot,
            contractAccountFundsSender,
            contractAccountFundsReceiver1,
            contractAccountFundsReceiver2);

    List<Transaction> transactions = new ArrayList<>();
    transactions.add(txToSendFundsToFundsSender);
    if (useCallCode) {
      transactions.add(txToInvokeTurnOnUseCallCodeInFundsSender);
    }
    transactions.add(txToInitiateCalls);

    ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(accounts)
            .transactions(transactions)
            .transactionProcessingResultValidator(
                TransactionProcessingResultValidator.EMPTY_VALIDATOR)
            .build();

    toyExecutionEnvironmentV2.run();
  }

  private static Stream<Arguments> contractForSLoadAndSStoreTestSource() {
    List<Arguments> arguments = new ArrayList<>();
    for (boolean toRoot : new boolean[] {true, false}) {
      for (boolean mustRevert : new boolean[] {true, false}) {
        for (CallCase callCase : CallCase.values()) {
          for (boolean useCallCode : new boolean[] {true, false}) {
            arguments.add(Arguments.of(toRoot, mustRevert, callCase, useCallCode));
          }
        }
      }
    }
    return arguments.stream();
  }

  // Support methods
  private static String invokeFundsSender(
      Address addressFundsSender,
      Address addressFundsReceiver1,
      Address addressFundsReceiver2,
      boolean mustRevert,
      CallCase callCase) {
    return "0x625fd839"
        + "000000000000000000000000"
        + addressFundsSender.toString().substring(2)
        + "000000000000000000000000"
        + addressFundsReceiver1.toString().substring(2)
        + "000000000000000000000000"
        + addressFundsReceiver2.toString().substring(2)
        + "000000000000000000000000000000000000000000000000000000000000000"
        + (mustRevert ? "1" : "0")
        + "000000000000000000000000000000000000000000000000000000000000000"
        + callCase.ordinal();
  }

  private static String transferFunds(
      Address addressFundsReceiver1,
      Address addressFundsReceiver2,
      boolean mustRevert,
      CallCase callCase) {
    return "0xc74c79b5"
        + "000000000000000000000000"
        + addressFundsReceiver1.toString().substring(2)
        + "000000000000000000000000"
        + addressFundsReceiver2.toString().substring(2)
        + "000000000000000000000000000000000000000000000000000000000000000"
        + (mustRevert ? "1" : "0")
        + "000000000000000000000000000000000000000000000000000000000000000"
        + callCase.ordinal();
  }
}
