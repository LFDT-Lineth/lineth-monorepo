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

package net.consensys.linea.zktracer.module.hub.fragment.transaction.system;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.module.hub.fragment.transaction.system.SystemTransactionType.SYSI_EIP_2935_HISTORICAL_HASH;

import net.consensys.linea.zktracer.Trace;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;

public class EIP2935TransactionFragment extends SystemTransactionFragment {
  final long previousBlockNumber;
  final short previousBlockNumberMod8191;
  final Bytes32 previousBlockhashOrZero;
  final boolean isGenesisBlock;

  public EIP2935TransactionFragment(
      long previousBlockNumber,
      short previousBlockNumberMod8191,
      Bytes32 previousBlockhashOrZero,
      boolean isGenesisBlock) {
    super(SYSI_EIP_2935_HISTORICAL_HASH);
    this.previousBlockNumber = previousBlockNumber;
    this.previousBlockNumberMod8191 = previousBlockNumberMod8191;
    this.previousBlockhashOrZero = previousBlockhashOrZero;
    this.isGenesisBlock = isGenesisBlock;
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    super.trace(trace);
    return trace
        .pTransactionEip2935(true)
        .pTransactionSystTxnData1(Bytes.ofUnsignedLong(previousBlockNumber))
        .pTransactionSystTxnData2(previousBlockNumberMod8191)
        .pTransactionSystTxnData3(previousBlockhashOrZero.slice(0, LLARGE))
        .pTransactionSystTxnData4(previousBlockhashOrZero.slice(LLARGE, LLARGE))
        .pTransactionSystTxnData5(isGenesisBlock);
  }
}
