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

package lineth.zktracer.module.blockdata.module;

import static lineth.zktracer.Trace.Blockdata.nROWS_DEPTH;
import static lineth.zktracer.opcode.OpCode.*;
import static lineth.zktracer.opcode.OpCode.BASEFEE;
import static lineth.zktracer.opcode.OpCode.CHAINID;
import static lineth.zktracer.opcode.OpCode.GASLIMIT;
import static lineth.zktracer.opcode.OpCode.PREVRANDAO;

import java.util.Map;
import lineth.zktracer.ChainConfig;
import lineth.zktracer.module.euc.Euc;
import lineth.zktracer.module.hub.Hub;
import lineth.zktracer.module.wcp.Wcp;
import lineth.zktracer.opcode.OpCode;
import org.apache.tuweni.bytes.Bytes;

public class ParisBlockData extends LondonBlockData {
  public ParisBlockData(
      Hub hub, Wcp wcp, Euc euc, ChainConfig chain, Map<Long, Bytes> blobBaseFees) {
    super(hub, wcp, euc, chain, blobBaseFees);
  }

  @Override
  protected OpCode[] setOpCodes() {
    return new OpCode[] {COINBASE, TIMESTAMP, NUMBER, PREVRANDAO, GASLIMIT, CHAINID, BASEFEE};
  }

  @Override
  protected int numberOfLinesPerBlock() {
    return nROWS_DEPTH;
  }
}
