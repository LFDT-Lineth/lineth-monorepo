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
package linea.plugin.acc.test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

import java.math.BigInteger;
import java.util.List;
import java.util.stream.IntStream;

import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.tests.acceptance.dsl.account.Accounts;
import org.junit.jupiter.api.Test;
import org.web3j.crypto.Credentials;
import org.web3j.protocol.Web3j;
import org.web3j.protocol.core.methods.response.EthSendTransaction;
import org.web3j.tx.RawTransactionManager;
import org.web3j.utils.Convert;

public class TransactionPoolDenialTest extends LineaPluginTestBase {

  private static final BigInteger GAS_PRICE = Convert.toWei("20", Convert.Unit.GWEI).toBigInteger();
  private static final BigInteger GAS_LIMIT = BigInteger.valueOf(210000);
  private static final BigInteger VALUE = BigInteger.ONE; // 1 wei

  @Override
  public List<String> getTestCliOptions() {
    return new TestCommandLineOptionsBuilder()
        .set("--plugin-linea-deny-list-path=", getResourcePath("/denyList.txt"))
        .build();
  }

  @Test
  public void senderOnDenyListCannotAddTransactionToPool() throws Exception {
    final Credentials notDenied = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    final Credentials denied = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY);
    final Web3j miner = minerNode.nodeRequests().eth();

    RawTransactionManager transactionManager = new RawTransactionManager(miner, denied, CHAIN_ID);
    EthSendTransaction transactionResponse =
        transactionManager.sendTransaction(GAS_PRICE, GAS_LIMIT, notDenied.getAddress(), "", VALUE);

    assertThat(transactionResponse.getTransactionHash()).isNull();
    assertThat(transactionResponse.getError().getMessage())
        .isEqualTo(
            "sender 0x627306090abab3a6e1400e9345bc60c78a8bef57 is blocked as appearing on the SDN or other legally prohibited list");
  }

  @Test
  public void transactionWithRecipientOnDenyListCannotBeAddedToPool() throws Exception {
    final Credentials notDenied = Credentials.create(Accounts.GENESIS_ACCOUNT_ONE_PRIVATE_KEY);
    final Credentials denied = Credentials.create(Accounts.GENESIS_ACCOUNT_TWO_PRIVATE_KEY);
    final Web3j miner = minerNode.nodeRequests().eth();

    RawTransactionManager transactionManager =
        new RawTransactionManager(miner, notDenied, CHAIN_ID);
    EthSendTransaction transactionResponse =
        transactionManager.sendTransaction(GAS_PRICE, GAS_LIMIT, denied.getAddress(), "", VALUE);

    assertThat(transactionResponse.getTransactionHash()).isNull();
    assertThat(transactionResponse.getError().getMessage())
        .isEqualTo(
            "recipient 0x627306090abab3a6e1400e9345bc60c78a8bef57 is blocked as appearing on the SDN or other legally prohibited list");
  }

  @Test
  public void transactionThatTargetPrecompileIsNotAccepted() {
    IntStream.rangeClosed(1, 9)
        .mapToObj(
            index ->
                accountTransactions.createTransfer(
                    accounts.getPrimaryBenefactor(),
                    accounts.createAccount(Address.precompiled(index)),
                    1,
                    BigInteger.valueOf(1)))
        .forEach(
            txWithPrecompileRecipient ->
                assertThatThrownBy(
                        () -> txWithPrecompileRecipient.execute(minerNode.nodeRequests()))
                    .hasMessage(
                        "Error sending transaction: destination address is a precompile address and cannot receive transactions"));
  }
}
