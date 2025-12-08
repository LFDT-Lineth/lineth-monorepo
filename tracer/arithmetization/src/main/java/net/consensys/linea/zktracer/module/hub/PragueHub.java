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

package net.consensys.linea.zktracer.module.hub;

import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP2935HistoricalHash;
import net.consensys.linea.zktracer.module.tables.bls.BlsRt;
import net.consensys.linea.zktracer.types.PublicInputs;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public class PragueHub extends CancunHub {
  public PragueHub(ChainConfig chain, PublicInputs publicInputs) {
    super(chain, publicInputs);
  }

  @Override
  protected BlsRt setBlsRt() {
    return new BlsRt();
  }

  @Override
  protected void traceSysiTransactions(WorldView world, ProcessableBlockHeader blockHeader) {
    super.traceSysiTransactions(world, blockHeader);
    state.incrementSysiTransactionNumber();
    new EIP2935HistoricalHash(this, world, blockHeader);
  }
}
