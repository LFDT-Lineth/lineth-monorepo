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

package net.consensys.linea.zktracer.module.rlptxn.phaseSection;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.Trace.Rlptxn.RLP_TXN_CT_MAX_ADDRESS;
import static net.consensys.linea.zktracer.module.rlpUtils.RlpUtils.BYTES16_PREFIX_ADDRESS;
import static net.consensys.linea.zktracer.module.rlputilsOld.Pattern.outerRlpSize;
import static net.consensys.linea.zktracer.types.AddressUtils.highPart;
import static net.consensys.linea.zktracer.types.AddressUtils.lowPart;
import static net.consensys.linea.zktracer.types.Utils.rightPadToBytes16;

import java.util.ArrayList;
import java.util.List;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.rlpUtils.InstructionByteStringPrefix;
import net.consensys.linea.zktracer.module.rlpUtils.InstructionBytes32;
import net.consensys.linea.zktracer.module.rlpUtils.RlpUtils;
import net.consensys.linea.zktracer.module.rlptxn.GenericTracedValue;
import net.consensys.linea.zktracer.module.rlputilsOld.Pattern;
import net.consensys.linea.zktracer.module.trm.Trm;
import net.consensys.linea.zktracer.types.TransactionProcessingMetadata;
import org.apache.tuweni.bytes.Bytes;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.AccessListEntry;
import org.hyperledger.besu.datatypes.Address;

public class AccessListPhaseSection extends PhaseSection {
  private int phaseSize;
  private int totalAddress;
  private int totalKeys;
  private final InstructionByteStringPrefix accessListRlpPrefix;
  private final List<AccessListEntrySubSection> entries;

  private static final short RLP_ADDRESS_BYTE_SIZE = 1 + Address.SIZE;

  public AccessListPhaseSection(RlpUtils rlpUtils, Trm trm, TransactionProcessingMetadata tx) {
    final List<AccessListEntry> accessList =
        tx.getBesuTransaction().getAccessList().orElse(List.of());

    final List<Integer> accessListTupleSizes = new ArrayList<>(accessList.size());
    for (AccessListEntry entry : accessList) {
      accessListTupleSizes.add(
          RLP_ADDRESS_BYTE_SIZE + outerRlpSize(33 * entry.storageKeys().size()));
    }

    phaseSize = accessListTupleSizes.stream().mapToInt(Pattern::outerRlpSize).sum();
    final InstructionByteStringPrefix accessListRlpPrefixCall =
        new InstructionByteStringPrefix(phaseSize, (byte) 0x00, true);
    accessListRlpPrefix = (InstructionByteStringPrefix) rlpUtils.call(accessListRlpPrefixCall);

    entries = new ArrayList<>(accessList.size());
    for (int i = 0; i < accessList.size(); i++) {
      entries.add(
          new AccessListEntrySubSection(
              rlpUtils, trm, accessList.get(i), accessListTupleSizes.get(i)));
    }
  }

  @Override
  protected void traceComputationsRows(
      Trace.Rlptxn trace, TransactionProcessingMetadata tx, GenericTracedValue tracedValues) {
    totalAddress = tx.numberOfWarmedAddresses();
    totalKeys = tx.numberOfWarmedStorageKeys();

    // Phase RlpPrefix
    traceTransactionConstantValues(trace, tracedValues);
    accessListRlpPrefix.traceRlpTxn(trace, tracedValues, true, true, true, 0);
    trace.pCmpAux1(phaseSize).pCmpAuxCcc1(totalAddress).pCmpAuxCcc2(Bytes.ofUnsignedInt(totalKeys));
    tracePostValues(trace, tracedValues);

    // trace each entry
    for (AccessListEntrySubSection entry : entries) {
      entry.trace(trace, tracedValues);
    }
  }

  @Override
  protected void traceIsPhaseX(Trace.Rlptxn trace) {
    trace.isAccessList(true);
  }

  @Override
  public int lineCount() {
    return 1 // 1 for transaction row
        + 1 // + 1 for global Rlp prefix
        + entries.stream().mapToInt(AccessListEntrySubSection::lineCount).sum(); // + the entries
  }

  private class AccessListEntrySubSection {
    private final InstructionByteStringPrefix entryRlpPrefix;
    private final Address address;
    private final InstructionByteStringPrefix keysRlpPrefix;
    private final List<InstructionBytes32> keys;

    private AccessListEntrySubSection(
        RlpUtils rlpUtils, Trm trm, AccessListEntry entry, int tupleByteSize) {
      final InstructionByteStringPrefix entryRlpPrefixCall =
          new InstructionByteStringPrefix(tupleByteSize, (byte) 0x00, true);
      entryRlpPrefix = (InstructionByteStringPrefix) rlpUtils.call(entryRlpPrefixCall);

      address = entry.address();
      // Note: add the address to the set of address to trim, only useful for SKIP transactions with
      // access list (no warming section in the HUB)
      trm.callTrimming(address);

      final InstructionByteStringPrefix keysRlpPrefixCall =
          new InstructionByteStringPrefix(33 * entry.storageKeys().size(), (byte) 0x00, true);
      keysRlpPrefix = (InstructionByteStringPrefix) rlpUtils.call(keysRlpPrefixCall);

      keys = new ArrayList<>(entry.storageKeys().size());
      for (Bytes32 key : entry.storageKeys()) {
        final InstructionBytes32 call = new InstructionBytes32(key);
        keys.add((InstructionBytes32) rlpUtils.call(call));
      }
    }

    private int lineCount() {
      return 1 // 1 for entry RlpPrefix
          + (RLP_TXN_CT_MAX_ADDRESS + 1) // 3 for the Address
          + 1 // 1 for the RlpPrefix of the list of keys
          + 3 * keys.size(); // 3 per keys
    }

    private void traceAccessListCountdownValues(
        Trace.Rlptxn trace, int tupleSize, int totalStorageForThisAddress) {
      trace
          .pCmpAuxCcc1(totalAddress)
          .pCmpAuxCcc2(Bytes.ofUnsignedInt(totalKeys))
          .pCmpAux1(phaseSize)
          .pCmpAux2(tupleSize)
          .pCmpAuxCcc3(totalStorageForThisAddress)
          .pCmpAuxCcc4(highPart(address))
          .pCmpAuxCcc5(lowPart(address));
    }

    public void trace(Trace.Rlptxn trace, GenericTracedValue tracedValues) {
      int tupleSize = entryRlpPrefix.byteStringLength();
      short totalStorageForThisAddress = (short) keys.size();

      totalAddress -= 1;

      // trace entry RlpPrefix
      traceTransactionConstantValues(trace, tracedValues);
      entryRlpPrefix.traceRlpTxn(trace, tracedValues, true, true, true, 0);
      phaseSize -= entryRlpPrefix.rlpPrefixByteSize();
      trace.isPrefixOfAccessListItem(true);
      traceAccessListCountdownValues(trace, tupleSize, totalStorageForThisAddress);
      tracePostValues(trace, tracedValues);

      // trace RLP(address)
      // RLP(address): first row: rlp prefix
      traceTransactionConstantValues(trace, tracedValues);
      trace
          .cmp(true)
          .isAccessListAddress(true)
          .ctMax(RLP_TXN_CT_MAX_ADDRESS)
          .pCmpTrmFlag(true)
          .pCmpExoData1(address.slice(0, 4))
          .pCmpExoData2(lowPart(address))
          .limbConstructed(true)
          .lt(true)
          .lx(true)
          .pCmpLimb(BYTES16_PREFIX_ADDRESS)
          .pCmpLimbSize(1);
      phaseSize -= 1;
      tupleSize -= 1;
      tracedValues.decrementLtAndLxSizeBy(1);
      traceAccessListCountdownValues(trace, tupleSize, totalStorageForThisAddress);
      tracePostValues(trace, tracedValues);

      // RLP(address): second row: address hi
      traceTransactionConstantValues(trace, tracedValues);
      trace
          .cmp(true)
          .isAccessListAddress(true)
          .ct(1)
          .ctMax(RLP_TXN_CT_MAX_ADDRESS)
          .limbConstructed(true)
          .lt(true)
          .lx(true)
          .pCmpLimb(rightPadToBytes16(address.slice(0, 4)))
          .pCmpLimbSize(4);
      phaseSize -= 4;
      tupleSize -= 4;
      tracedValues.decrementLtAndLxSizeBy(4);
      traceAccessListCountdownValues(trace, tupleSize, totalStorageForThisAddress);
      tracePostValues(trace, tracedValues);

      // RLP(address):third row: address lo
      traceTransactionConstantValues(trace, tracedValues);
      trace
          .cmp(true)
          .isAccessListAddress(true)
          .ct(2)
          .ctMax(RLP_TXN_CT_MAX_ADDRESS)
          .limbConstructed(true)
          .lt(true)
          .lx(true)
          .pCmpLimb(lowPart(address))
          .pCmpLimbSize(LLARGE);
      phaseSize -= LLARGE;
      tupleSize -= LLARGE;
      tracedValues.decrementLtAndLxSizeBy(LLARGE);
      traceAccessListCountdownValues(trace, tupleSize, totalStorageForThisAddress);
      tracePostValues(trace, tracedValues);

      // RLP prefix (keys)
      traceTransactionConstantValues(trace, tracedValues);
      keysRlpPrefix.traceRlpTxn(trace, tracedValues, true, true, true, 0);
      phaseSize -= keysRlpPrefix.rlpPrefixByteSize();
      tupleSize -= keysRlpPrefix.rlpPrefixByteSize();
      trace.isPrefixOfStorageKeyList(true);
      traceAccessListCountdownValues(trace, tupleSize, totalStorageForThisAddress);
      tracePostValues(trace, tracedValues);

      // optionally trace RLP(key)
      for (InstructionBytes32 key : keys) {
        totalKeys -= 1;
        totalStorageForThisAddress -= 1;

        // RLP(key): first row: rlp prefix
        traceTransactionConstantValues(trace, tracedValues);
        key.traceRlpTxn(trace, tracedValues, true, true, true, 0);
        phaseSize -= 1;
        tupleSize -= 1;
        traceAccessListCountdownValues(trace, tupleSize, totalStorageForThisAddress);
        tracePostValues(trace, tracedValues);

        // RLP(key): second row: key hi
        traceTransactionConstantValues(trace, tracedValues);
        key.traceRlpTxn(trace, tracedValues, true, true, true, 1);
        phaseSize -= LLARGE;
        tupleSize -= LLARGE;
        traceAccessListCountdownValues(trace, tupleSize, totalStorageForThisAddress);
        tracePostValues(trace, tracedValues);

        // RLP(key): second row: key lo
        traceTransactionConstantValues(trace, tracedValues);
        key.traceRlpTxn(trace, tracedValues, true, true, true, 2);
        phaseSize -= LLARGE;
        tupleSize -= LLARGE;
        traceAccessListCountdownValues(trace, tupleSize, totalStorageForThisAddress);
        tracePostValues(trace, tracedValues);
      }
    }
  }
}
