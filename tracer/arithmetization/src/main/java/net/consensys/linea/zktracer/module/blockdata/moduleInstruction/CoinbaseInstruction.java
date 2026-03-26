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
package net.consensys.linea.zktracer.module.blockdata.moduleInstruction;

import static net.consensys.linea.zktracer.Trace.Blockdata.nROWS_CB;

import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.blockdata.BlockDataExoCall;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.plugin.data.BlockHeader;

public class CoinbaseInstruction extends BlockDataInstruction {

  public CoinbaseInstruction(
      ChainConfig chain,
      Hub hub,
      Wcp wcp,
      Euc euc,
      BlockHeader blockHeader,
      BlockHeader prevBlockHeader,
      long firstBlockNumber) {
    super(OpCode.COINBASE, chain, hub, wcp, euc, blockHeader, prevBlockHeader, firstBlockNumber);
  }

  public void handle() {
    data =
        EWord.ofHexString(
            this.hub.coinbaseAddressOfRelativeBlock(this.relBlock).getBytes().toHexString());
    // row i
    exoCalls[0] = BlockDataExoCall.callToLT(this.wcp, data, POWER_256_20);
  }

  public int nbRows() {
    return nROWS_CB;
  }

  public void traceInstruction(Trace.Blockdata trace) {
    trace.isCoinbase(true);
  }
}
