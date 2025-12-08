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

package net.consensys.linea.zktracer.module.hub.section.txInitializationSection;

import static net.consensys.linea.zktracer.module.hub.AccountSnapshot.canonical;

import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.hyperledger.besu.evm.worldstate.WorldView;

public class ShanghaiInitializationSection extends LondonInitializationSection {
  public ShanghaiInitializationSection(Hub hub, WorldView world) {
    super(hub, world);
  }

  @Override
  protected void addCoinbaseWarmingFragment() {
    this.addFragment(coinbaseWarmingAccountFragment);
  }

  @Override
  protected AccountFragment makeCoinbaseWarmingFragment(
      final Hub hub, final WorldView world, final TransactionProcessingMetadata tx) {
    final AccountSnapshot coinbase =
        canonical(hub, world, hub.coinbaseAddress(), tx.isCoinbasePreWarmed());
    return accountFragmentFactory.makeWithTrm(
        coinbase,
        coinbase.deepCopy().turnOnWarmth(),
        coinbase.address(),
        DomSubStampsSubFragment.standardDomSubStamps(getHubStamp(), domSubOffset()),
        TransactionProcessingType.USER);
  }

  @Override
  protected boolean senderWarmthAtGasPayment(final TransactionProcessingMetadata tx) {
    return tx.isSenderPreWarmed() || tx.senderIsCoinbase();
  }

  @Override
  protected boolean recipientWarmthAtValueReception(TransactionProcessingMetadata tx) {
    return tx.isRecipientPreWarmed() || tx.recipientIsCoinbase();
  }
}
