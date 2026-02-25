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

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.Trace.*;
import static net.consensys.linea.zktracer.module.hub.TransactionProcessingType.SYSI;
import static net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragment.systemTransactionStoring;
import static net.consensys.linea.zktracer.types.Conversions.bigIntegerToBytes16;
import static net.consensys.linea.zktracer.types.Conversions.longToUnsignedBigInteger;

import java.math.BigInteger;
import net.consensys.linea.zktracer.module.hub.AccountSnapshot;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;
import net.consensys.linea.zktracer.module.hub.fragment.storage.StorageFragment;
import net.consensys.linea.zktracer.module.hub.fragment.transaction.system.Eip4788TransactionFragment;
import net.consensys.linea.zktracer.module.hub.section.TraceSection;
import net.consensys.linea.zktracer.types.AddressUtils;
import net.consensys.linea.zktracer.types.EWord;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.apache.tuweni.units.bigints.UInt256;
import org.hyperledger.besu.datatypes.Address;
import org.hyperledger.besu.evm.worldstate.WorldView;
import org.hyperledger.besu.plugin.data.ProcessableBlockHeader;

public class EIP4788BeaconBlockRootSection extends TraceSection {

  public static final short NB_ROWS_HUB_SYSI_EIP_4788 = 5;

  public static final Address EIP4788_BEACONROOT_ADDRESS =
      AddressUtils.addressFromBytes(
          Bytes.concatenate(
              Bytes.minimalBytes(BEACON_ROOTS_ADDRESS_HI),
              bigIntegerToBytes16(BEACON_ROOTS_ADDRESS_LO)));

  public static final BigInteger HISTORY_BUFFER_LENGTH_BI =
      BigInteger.valueOf(HISTORY_BUFFER_LENGTH);

  public EIP4788BeaconBlockRootSection(
      Hub hub, WorldView world, ProcessableBlockHeader blockHeader) {
    super(hub, NB_ROWS_HUB_SYSI_EIP_4788);
    final AccountSnapshot beaconrootAccount =
        AccountSnapshot.canonical(hub, world, EIP4788_BEACONROOT_ADDRESS, false);
    final BigInteger timestamp = longToUnsignedBigInteger(blockHeader.getTimestamp());
    final boolean currentBlockIsGenesisBlock = blockHeader.getNumber() == 0;
    final boolean isNonTrivialOperation =
        !currentBlockIsGenesisBlock && !beaconrootAccount.code().isEmpty();
    checkState(blockHeader.getParentBeaconBlockRoot().isPresent(), "Missing parentBeaconBlockRoot");
    checkState(
        !currentBlockIsGenesisBlock || blockHeader.getParentBeaconBlockRoot().get().isZero(),
        "Genesis block must have a zero parentBeaconBlockRoot");
    final Bytes32 beaconRoot = blockHeader.getParentBeaconBlockRoot().get();

    final Eip4788TransactionFragment transactionFragment =
        new Eip4788TransactionFragment(timestamp, beaconRoot, currentBlockIsGenesisBlock);
    fragments().add(transactionFragment);
    hub.txnData().callTxnDataForSystemTransaction(transactionFragment.type());

    final AccountFragment accountFragment =
        hub.factories()
            .accountFragment()
            .makeWithTrm(
                beaconrootAccount,
                beaconrootAccount,
                EIP4788_BEACONROOT_ADDRESS,
                DomSubStampsSubFragment.standardDomSubStamps(hubStamp(), 1),
                SYSI);
    fragments().add(accountFragment);

    if (isNonTrivialOperation) {
      final EWord keyTimestamp = EWord.of(timestamp.mod(HISTORY_BUFFER_LENGTH_BI));
      final StorageFragment storingTimestamp =
          systemTransactionStoring(
              hub,
              EIP4788_BEACONROOT_ADDRESS,
              keyTimestamp,
              EWord.of(
                  world
                      .get(EIP4788_BEACONROOT_ADDRESS)
                      .getStorageValue(UInt256.fromBytes(keyTimestamp))),
              EWord.of(timestamp),
              2);
      fragments().add(storingTimestamp);

      final EWord keyBeaconRoot = keyTimestamp.add(HISTORY_BUFFER_LENGTH);
      final StorageFragment storingBeaconroot =
          systemTransactionStoring(
              hub,
              EIP4788_BEACONROOT_ADDRESS,
              keyBeaconRoot,
              EWord.of(
                  world
                      .get(EIP4788_BEACONROOT_ADDRESS)
                      .getStorageValue(UInt256.fromBytes(keyBeaconRoot))),
              EWord.of(beaconRoot),
              3);
      fragments().add(storingBeaconroot);
    }

    fragments().add(ContextFragment.readZeroContextData(hub));
  }
}
