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

package net.consensys.linea.testing;

import java.util.ArrayList;
import java.util.List;
import lombok.Builder;
import net.consensys.linea.testing.ToyTransaction.ToyTransactionBuilder;
import org.hyperledger.besu.ethereum.core.Transaction;

@Builder
public class ToyMultiTransaction {

  /** Customizations applied to the Lombok generated builder. */
  public static class ToyMultiTransactionBuilder {

    /**
     * Builder method returning an instance of {@link List<Transaction>}.
     *
     * @return an instance of {@link List<Transaction>}
     */
    public List<Transaction> build(
        List<ToyTransactionBuilder> toyTxBuilders, ToyAccount senderAccount) {
      long senderAccountNonce = senderAccount.getNonce();
      List<Transaction> results = new ArrayList<>();
      for (ToyTransactionBuilder toyTxBuilder : toyTxBuilders) {
        results.add(toyTxBuilder.nonce(senderAccountNonce).build());
        senderAccountNonce++;
      }
      return results;
    }
  }
}
