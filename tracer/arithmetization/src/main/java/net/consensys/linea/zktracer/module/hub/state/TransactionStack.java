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

package net.consensys.linea.zktracer.module.hub.state;

import static com.google.common.base.Preconditions.checkState;

import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.container.stacked.StackedList;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.section.TxInitializationSection;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.datatypes.Transaction;
import org.hyperledger.besu.evm.worldstate.WorldView;

@Getter
public class TransactionStack {
  @Accessors(fluent = true)
  private final StackedList<TransactionProcessingMetadata> transactions = new StackedList<>();

  @Accessors(fluent = true)
  private int currentAbsNumber;

  @Accessors(fluent = true)
  private int relativeTransactionNumber;

  @Setter public TxInitializationSection initializationSection;

  public TransactionProcessingMetadata current() {
    return transactions.getLast();
  }

  /* WARN: can't be called if currentAbsNumber == 1*/
  public TransactionProcessingMetadata previous() {
    return transactions.get(transactions.size() - 2);
  }

  public TransactionProcessingMetadata getByAbsoluteTransactionNumber(final int id) {
    return transactions.get(id - 1);
  }

  public void commitTransactionBundle() {
    transactions.commitTransactionBundle();
  }

  public void popTransactionBundle() {
    final int numberOfTransactionToPop = transactions.operationsInTransactionBundle().size();
    transactions.popTransactionBundle();
    currentAbsNumber -= numberOfTransactionToPop;
    relativeTransactionNumber -= numberOfTransactionToPop;
    checkState(
        relativeTransactionNumber >= 0,
        "TransactionStack: relativeTransactionNumber = %s < 0",
        relativeTransactionNumber);
  }

  public void resetBlock() {
    relativeTransactionNumber = 0;
  }

  public void enterTransaction(final Hub hub, final WorldView world, final Transaction tx) {
    currentAbsNumber += 1;
    relativeTransactionNumber += 1;

    transactions()
        .add(
            new TransactionProcessingMetadata(
                hub, world, tx, relativeTransactionNumber, currentAbsNumber));
  }

  public void setCodeFragmentIndex(Hub hub) {
    for (TransactionProcessingMetadata tx : transactions.getAll()) {
      final int cfi =
          tx.requiresCfiUpdate()
              ? hub.getCodeFragmentIndexByMetaData(
                  tx.getEffectiveRecipient(),
                  tx.getUpdatedRecipientAddressDeploymentNumberAtTransactionStart(),
                  tx.isUpdatedRecipientAddressDeploymentStatusAtTransactionStart(),
                  tx.getDelegationNumberAtTransactionStart())
              : 0;
      tx.setCodeFragmentIndex(cfi);
    }
  }

  public long getAccumulativeGasUsedInBlockBeforeTxStart() {
    return this.relativeTransactionNumber == 1 ? 0 : this.previous().getAccumulatedGasUsedInBlock();
  }
}
