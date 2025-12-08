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

package net.consensys.linea.zktracer.module.hub.section.skip;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.transients.Transients;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.evm.worldstate.WorldView;

public class CancunTxSkipSection extends LondonTxSkipSection {
  public CancunTxSkipSection(
      Hub hub,
      WorldView world,
      TransactionProcessingMetadata transactionProcessingMetadata,
      Transients transients) {
    super(hub, world, transactionProcessingMetadata, transients);
  }

  @Override
  protected short senderDomStamp() {
    return 1;
  }

  @Override
  protected short recipientDomStamp() {
    return 2;
  }

  @Override
  protected short coinbaseDomStamp() {
    return 3;
  }

  @Override
  protected void addFragments(
      TransactionProcessingMetadata txMetadata,
      AccountFragment senderAccountFragment,
      AccountFragment recipientAccountFragment,
      AccountFragment coinbaseAccountFragment) {
    this.addFragment(txMetadata.userTransactionFragment());
    this.addFragment(senderAccountFragment);
    this.addFragment(recipientAccountFragment);
    this.addFragment(coinbaseAccountFragment);
    this.addFragment(ContextFragment.readZeroContextData(commonValues.hub));
  }
}
