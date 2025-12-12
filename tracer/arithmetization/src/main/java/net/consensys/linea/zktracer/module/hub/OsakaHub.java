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
import net.consensys.linea.zktracer.module.blake2fmodexpdata.*;
import net.consensys.linea.zktracer.module.txndata.TxnData;
import net.consensys.linea.zktracer.module.txndata.osaka.OsakaTxnData;
import net.consensys.linea.zktracer.types.PublicInputs;

public class OsakaHub extends PragueHub {
  public OsakaHub(ChainConfig chain, PublicInputs publicInputs) {
    super(chain, publicInputs);
  }

  @Override
  protected TxnData setTxnData() {
    return new OsakaTxnData(this, wcp(), euc());
  }
}
