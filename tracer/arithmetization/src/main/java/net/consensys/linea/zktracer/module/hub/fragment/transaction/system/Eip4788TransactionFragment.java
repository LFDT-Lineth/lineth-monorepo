/*
 * Copyright Consensys Software Inc.
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

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.module.hub.fragment.transaction.system.SystemTransactionType.SYSI_EIP_4788_BEACON_BLOCK_ROOT;
import static net.consensys.linea.zktracer.module.hub.section.systemTransaction.EIP4788BeaconBlockRootSection.HISTORY_BUFFER_LENGTH_BI;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes;

import java.math.BigInteger;
import net.consensys.linea.zktracer.Trace;
import org.apache.tuweni.bytes.Bytes32;

public class Eip4788TransactionFragment extends SystemTransactionFragment {

  final boolean isGenesisBlock;
  final BigInteger timestamp;
  final Bytes32 beaconroot;

  public Eip4788TransactionFragment(
      BigInteger timestamp, Bytes32 beaconroot, boolean isGenesisBlock) {
    super(SYSI_EIP_4788_BEACON_BLOCK_ROOT);
    this.timestamp = timestamp;
    this.beaconroot = beaconroot;
    this.isGenesisBlock = isGenesisBlock;
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    super.trace(trace);
    return trace
        .pTransactionEip4788(true)
        .pTransactionSystTxnData1(bigIntegerToBytes(timestamp))
        .pTransactionSystTxnData2((timestamp.mod(HISTORY_BUFFER_LENGTH_BI)).longValueExact())
        .pTransactionSystTxnData3(beaconroot.slice(0, LLARGE))
        .pTransactionSystTxnData4(beaconroot.slice(LLARGE, LLARGE))
        .pTransactionSystTxnData5(isGenesisBlock);
  }
}
