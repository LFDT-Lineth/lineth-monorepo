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

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.TWOFIFTYSIX_TO_THE_TWENTY;
import static net.consensys.linea.zktracer.opcode.OpCode.*;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import java.math.BigInteger;
import net.consensys.linea.zktracer.ChainConfig;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.blockdata.BlockDataExoCall;
import net.consensys.linea.zktracer.module.euc.Euc;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.wcp.Wcp;
import net.consensys.linea.zktracer.opcode.OpCode;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.plugin.data.BlockHeader;

public abstract class BlockDataInstruction {
  public final OpCode opCode;
  public final Hub hub;
  public final Wcp wcp;
  public final Euc euc;
  public final BlockHeader blockHeader;
  public final BlockHeader prevBlockHeader;
  public final ChainConfig chainConfig;
  public final long firstBlockNumber;

  protected EWord data;
  public final int relBlock;

  public static final EWord POWER_256_20 = EWord.of(TWOFIFTYSIX_TO_THE_TWENTY);
  public static final EWord POWER_256_8 = EWord.of(BigInteger.ONE.shiftLeft(8 * 8));

  /** Store all wcp and euc computations with params and results */
  public final BlockDataExoCall[] exoCalls = new BlockDataExoCall[nbRows()];

  public BlockDataInstruction(
      OpCode opCode,
      ChainConfig chain,
      Hub hub,
      Wcp wcp,
      Euc euc,
      BlockHeader blockHeader,
      BlockHeader prevBlockHeader,
      long firstBlockNumber) {
    this.opCode = opCode;
    this.hub = hub;
    this.wcp = wcp;
    this.euc = euc;
    this.blockHeader = blockHeader;
    this.chainConfig = chain;
    this.firstBlockNumber = firstBlockNumber;
    this.prevBlockHeader = prevBlockHeader;
    this.relBlock = (int) (blockHeader.getNumber() - firstBlockNumber + 1);
  }

  public abstract void handle();

  public abstract int nbRows();

  public abstract void traceInstruction(Trace.Blockdata trace);

  public void trace(
      Trace.Blockdata trace, boolean shouldTraceTsAndNb, boolean shouldTraceRelTxNumMax) {
    int nbRows = nbRows();
    for (short ct = 0; ct < nbRows; ct++) {
      trace
          .iomf(true)
          .ctMax(nbRows - 1)
          .ct(ct)
          .inst(opCode.unsignedByteValue()) // not fork dependant
          .coinbaseHi(hub.coinbaseAddressOfRelativeBlock(relBlock).getBytes().slice(0, 4).toLong())
          .coinbaseLo(hub.coinbaseAddressOfRelativeBlock(relBlock).getBytes().slice(4, LLARGE))
          .blockGasLimit(Bytes.ofUnsignedLong(blockHeader.getGasLimit()))
          .basefee(bigIntegerToBytes(blockHeader.getBaseFee().get().getAsBigInteger()))
          .firstBlockNumber(firstBlockNumber)
          .relBlock((short) relBlock)
          .dataHi(data.hi())
          .dataLo(data.lo())
          .arg1Hi(exoCalls[ct].arg1Hi())
          .arg1Lo(exoCalls[ct].arg1Lo())
          .arg2Hi(exoCalls[ct].arg2Hi())
          .arg2Lo(exoCalls[ct].arg2Lo())
          .res(exoCalls[ct].res())
          .exoInst(exoCalls[ct].instruction())
          .wcpFlag(exoCalls[ct].wcpFlag())
          .eucFlag(exoCalls[ct].eucFlag());
      traceInstruction(trace);
      trace
          .timestamp(Bytes.ofUnsignedLong(blockHeader.getTimestamp()))
          .number(blockHeader.getNumber());
      trace.fillAndValidateRow();
    }
  }
}
