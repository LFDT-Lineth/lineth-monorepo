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

import static net.consensys.linea.zktracer.Trace.Blockdata.nROWS_NB;

import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.blockdata.BlockDataExoCall;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.EWord;
import org.hyperledger.besu.plugin.data.BlockHeader;

public class NumberInstruction extends BlockDataInstruction {

  private final boolean firstBlockInConflation;

  public NumberInstruction(
      ChainConfig chain,
      Hub hub,
      Wcp wcp,
      Euc euc,
      BlockHeader blockHeader,
      BlockHeader prevBlockHeader,
      long firstBlockNumber) {
    super(OpCode.NUMBER, chain, hub, wcp, euc, blockHeader, prevBlockHeader, firstBlockNumber);
    this.firstBlockInConflation = (blockHeader.getNumber() == firstBlockNumber);
  }

  public void handle() {
    data = EWord.of(blockHeader.getNumber());

    exoCalls[0] = BlockDataExoCall.callToIsZero(this.wcp, EWord.of(firstBlockNumber));

    // Default values
    exoCalls[1] = BlockDataExoCall.builder().build();
    // row i
    if (firstBlockInConflation) {
      exoCalls[1] = BlockDataExoCall.callToLT(this.wcp, data, POWER_256_8);
    }
  }

  public int nbRows() {
    return nROWS_NB;
  }

  public void traceInstruction(Trace.Blockdata trace) {
    trace.isNumber(true);
  }
}
