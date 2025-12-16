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

package net.consensys.linea.zktracer.module.hub.fragment.account;

import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.isUserTransaction;

import java.util.Optional;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;

public class CancunAccountFragment extends LondonAccountFragment {
  private final TransactionProcessingMetadata tx;
  private final TransactionProcessingType txType;

  public CancunAccountFragment(
      Hub hub,
      AccountSnapshot oldState,
      AccountSnapshot newState,
      Optional<Bytes> addressToTrim,
      DomSubStampsSubFragment domSubStampsSubFragment,
      TransactionProcessingType txProcessingType) {
    super(hub, oldState, newState, addressToTrim, domSubStampsSubFragment, txProcessingType);
    txType = txProcessingType;
    tx = isUserTransaction(txType) ? hub.txStack().current() : null;
    if (isUserTransaction(txType)) {
      tx.updateHadCodeInitially(
          oldState.address(),
          domSubStampsSubFragment.domStamp(),
          domSubStampsSubFragment.subStamp(),
          oldState().tracedHasCode());
    }
  }

  public boolean shouldBeMarkedForDeletion() {
    return !transactionProcessingMetadata.hadCodeInitiallyMap().get(oldState().address()).hadCode();
  }

  @Override
  void traceMarkedForSelfDestruct(Trace.Hub trace) {
    // Those columns disappear in Cancun
  }

  @Override
  void traceMarkedForDeletion(Trace.Hub trace) {
    trace
        .pAccountMarkedForDeletion(markedForDeletion)
        .pAccountMarkedForDeletionNew(markedForDeletionNew);
  }

  @Override
  void traceHadCodeInitially(Trace.Hub trace) {
    trace.pAccountHadCodeInitially(
        isUserTransaction(txType)
            ? tx.hadCodeInitiallyMap().get(oldState().address()).hadCode()
            : oldState().tracedHasCode());
  }
}
