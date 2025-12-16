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
package net.consensys.linea.zktracer.instructionprocessing.createTests.advanced;

import static net.consensys.linea.zktracer.instructionprocessing.createTests.advanced.AdvancedCreate2ScenarioValue.*;
import static net.consensys.linea.zktracer.instructionprocessing.createTests.advanced.ScenarioUtils.getTransactions;
import static net.consensys.linea.zktracer.instructionprocessing.utilities.MonoOpCodeSmcs.userAccount;

import java.math.BigInteger;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Stream;
import net.consensys.linea.reporting.TracerTestBase;
import net.consensys.linea.testing.SmartContractUtils;
import net.consensys.linea.testing.ToyAccount;
import net.consensys.linea.testing.ToyExecutionEnvironmentV2;
import net.consensys.linea.testing.generated.Factory;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.datatypes.Wei;
import org.hyperledger.besu.ethereum.core.Transaction;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestInfo;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;
import org.web3j.abi.datatypes.generated.Uint256;

/**
 * This class implements the tests specified in <a
 * href="https://github.com/Consensys/linea-tracer/issues/2016">Tracer Issue #2016</a>.
 *
 * <p>The gist of the tests is to create situations where storage gets touched without any state
 * change resulting from it. Tests are structured as follows:
 *
 * <ul>
 *   <li>Transaction 1: SMC <b>A</b> deploys SMC <b>B</b> and may or may not touch / modify
 *       <b>B</b>'s storage in the process
 *   <li>Transaction 2: SMC <b>A</b> calls the newly deployed SMC <b>B</b>. The underlying method
 *       call performs a sequence of actions which include:
 *       <ul>
 *         <li>Pre-warming one or more of <b>B</b>'s storage slots
 *         <li>Touching / reading storage from <b>B</b>
 *         <li>Selfdestructing <b>B</b>
 *       </ul>
 *       Resuming execution in <b>A</b> we may or may not <b>REVERT</b>.
 * </ul>
 */
public class StorageNoOpTests extends TracerTestBase {

  static final Long gasLimit = 8000000L;
  static final Wei defaultBalance = Wei.fromEth(13L);
  static ToyAccount factorySmc =
      ToyAccount.builder()
          .address(Address.fromHexString("0x0add70fc7ea7e2"))
          .balance(defaultBalance)
          .nonce(1777L)
          .code(SmartContractUtils.getSolidityContractRuntimeByteCode(Factory.class))
          .build();

  @Test
  public void simpleTest(TestInfo testInfo) {

    testBody(
        TouchStorage.TOUCH_STORAGE,
        ModifyStorage.MODIFY_STORAGE,
        SelfDestruct.SELF_DESTRUCT,
        Revert.DONT_REVERT,
        testInfo);
  }

  @ParameterizedTest
  @MethodSource("getParameters")
  public void
      deployContractInFirstTransactionAndCallMethodSecondTransactionPotentiallySelfDestructing(
          TouchStorage touchStorage,
          ModifyStorage modifyStorage,
          SelfDestruct selfdestruct,
          Revert revert,
          TestInfo testInfo) {
    testBody(touchStorage, modifyStorage, selfdestruct, revert, testInfo);
  }

  public static Stream<Arguments> getParameters() {

    List<Arguments> arguments = new ArrayList<>();
    for (TouchStorage touchStorage : TouchStorage.values()) {
      for (ModifyStorage modifyStorage : ModifyStorage.values()) {
        for (SelfDestruct selfdestruct : SelfDestruct.values()) {
          for (Revert revert : Revert.values()) {
            arguments.add(Arguments.of(touchStorage, modifyStorage, selfdestruct, revert));
          }
        }
      }
    }
    return arguments.stream();
  }

  private void testBody(
      TouchStorage touchStorage,
      ModifyStorage modifyStorage,
      SelfDestruct selfdestruct,
      Revert revert,
      TestInfo testInfo) {

    // preparing payload for the first transaction
    Uint256 salt = new Uint256(BigInteger.valueOf(0xaaff11L));
    Bytes deployPayload = CustomStorageNoOpPayload.deploy(salt);
    Bytes callMainMethod =
        CustomStorageNoOpPayload.callMain(
            touchStorage == TouchStorage.TOUCH_STORAGE,
            modifyStorage == ModifyStorage.MODIFY_STORAGE,
            selfdestruct == SelfDestruct.SELF_DESTRUCT,
            revert == Revert.REVERT);
    // for some reason the solidity code does not allow to pass a nonzero value to the
    // transaction; we thus provide zero Wei (NONE) to both transactions
    List<Transaction> transactions =
        getTransactions(
            factorySmc, userAccount, List.of(deployPayload, callMainMethod), List.of(NONE, NONE));

    final ToyExecutionEnvironmentV2 toyExecutionEnvironmentV2 =
        ToyExecutionEnvironmentV2.builder(chainConfig, testInfo)
            .accounts(List.of(userAccount, factorySmc))
            .transactions(transactions)
            .build();
    toyExecutionEnvironmentV2.run();
  }

  private enum TouchStorage {
    TOUCH_STORAGE,
    DONT_TOUCH_STORAGE
  }

  private enum ModifyStorage {
    MODIFY_STORAGE,
    DONT_MODIFY_STORAGE
  }

  private enum SelfDestruct {
    SELF_DESTRUCT,
    DONT_SELF_DESTRUCT
  }

  private enum Revert {
    REVERT,
    DONT_REVERT
  }
}
