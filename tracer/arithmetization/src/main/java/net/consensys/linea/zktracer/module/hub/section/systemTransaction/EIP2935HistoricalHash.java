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

package net.consensys.linea.zktracer.module.hub.section.systemTransaction;

import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.SYSI;
import static net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragment.systemTransactionStoring;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes16;

import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragment;
import net.consensys.linea.zktracer.module.hub.fragment.transaction.system.EIP2935TransactionFragment;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.types.AddressUtils;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public class EIP2935HistoricalHash extends TraceSection {

  public static final short NB_ROWS_HUB_SYSI_EIP_2935 = 4;

  public static final Address EIP2935_HISTORY_STORAGE_ADDRESS =
      AddressUtils.addressFromBytes(
          Bytes.concatenate(
              Bytes.minimalBytes(HISTORY_STORAGE_ADDRESS_HI),
              bigIntegerToBytes16(HISTORY_STORAGE_ADDRESS_LO)));

  public EIP2935HistoricalHash(final Hub hub, WorldView world, ProcessableBlockHeader blockHeader) {
    super(hub, NB_ROWS_HUB_SYSI_EIP_2935);
    final long blockNumber = blockHeader.getNumber();
    final boolean currentBlockIsGenesis = blockNumber == 0;
    // Note: this is supposed to be useless in prod, as BLOCKHASH must have already loaded the
    // historical blockhashes
    if (!currentBlockIsGenesis) {
      hub.blockhash().callBlockhashForParent(blockHeader);
    }
    final long previousBlockNumber = currentBlockIsGenesis ? 0 : blockNumber - 1;
    final short previousBlockNumberMod8191 = (short) (previousBlockNumber % HISTORY_SERVE_WINDOW);
    final AccountSnapshot blockhashHistoryAccount =
        AccountSnapshot.canonical(hub, world, EIP2935_HISTORY_STORAGE_ADDRESS, false);
    final boolean isNonTrivialOperation =
        !currentBlockIsGenesis && !blockhashHistoryAccount.code().isEmpty();

    final Bytes32 previousBlockhashOrZero =
        currentBlockIsGenesis ? Bytes32.ZERO : blockHeader.getParentHash();

    final EIP2935TransactionFragment transactionFragment =
        new EIP2935TransactionFragment(
            previousBlockNumber,
            previousBlockNumberMod8191,
            previousBlockhashOrZero,
            currentBlockIsGenesis);
    fragments().add(transactionFragment);
    hub.txnData().callTxnDataForSystemTransaction(transactionFragment.type());

    final AccountFragment accountFragment =
        hub.factories()
            .accountFragment()
            .makeWithTrm(
                blockhashHistoryAccount,
                blockhashHistoryAccount,
                EIP2935_HISTORY_STORAGE_ADDRESS,
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp(), 1),
                SYSI);
    fragments().add(accountFragment);

    if (isNonTrivialOperation) {
      final EWord key = EWord.of(previousBlockNumberMod8191);
      final StorageFragment storingBlockhash =
          systemTransactionStoring(
              hub,
              EIP2935_HISTORY_STORAGE_ADDRESS,
              key,
              EWord.of(
                  world
                      .get(EIP2935_HISTORY_STORAGE_ADDRESS)
                      .getStorageValue(UInt256.fromBytes(key))),
              EWord.of(previousBlockhashOrZero),
              2);
      fragments().add(storingBlockhash);
    }

    fragments().add(ContextFragment.readZeroContextData(hub));
  }
}
