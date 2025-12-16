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

import java.util.Optional;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.TransactionProcessingType;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import org.apache.tuweni.bytes.Bytes;

public class LondonAccountFragment extends AccountFragment {

  public LondonAccountFragment(
      Hub hub,
      AccountSnapshot oldState,
      AccountSnapshot newState,
      Optional<Bytes> addressToTrim,
      DomSubStampsSubFragment domSubStampsSubFragment,
      TransactionProcessingType txProcessingType) {

    super(hub, oldState, newState, addressToTrim, domSubStampsSubFragment, txProcessingType);
  }

  // Fragment always wiped in London
  public boolean shouldBeMarkedForDeletion() {
    return true;
  }
  ;

  @Override
  void traceMarkedForSelfDestruct(Trace.Hub trace) {
    trace
        .pAccountMarkedForSelfdestruct(markedForDeletion)
        .pAccountMarkedForSelfdestructNew(markedForDeletionNew);
  }

  @Override
  void traceMarkedForDeletion(Trace.Hub trace) {
    // Those columns appear in Cancun
  }

  @Override
  void traceHadCodeInitially(Trace.Hub trace) {
    // This column appears in Cancun
  }
}
