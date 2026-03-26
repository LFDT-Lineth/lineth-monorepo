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

package net.consensys.linea.zktracer.module.hub.fragment;

import static net.consensys.linea.zktracer.Trace.LLARGE;
import static net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment.revertWithCurrentDomSubStamps;
import static net.consensys.linea.zktracer.module.hub.fragment.DomSubStampsSubFragment.standardDomSubStamps;
import static net.consensys.linea.zktracer.types.Conversions.bytesToLong;

import lombok.RequiredArgsConstructor;
import net.consensys.linea.zktracer.Trace;
import org.apache.tuweni.bytes.Bytes32;
import org.hyperledger.besu.datatypes.Address;

@RequiredArgsConstructor
public final class TransientFragment implements TraceFragment {

  final DomSubStampsSubFragment domSubStampsSubFragment;
  final Address address;
  final Bytes32 key;
  final Bytes32 valueCurr;
  final Bytes32 valueNext;

  public static TransientFragment tload(
      final int hubStamp, final Address address, final Bytes32 key, final Bytes32 valueCurrent) {
    return new TransientFragment(
        standardDomSubStamps(hubStamp, 0), address, key, valueCurrent, valueCurrent);
  }

  public static TransientFragment tstoreDoing(
      final int hubStamp,
      final Address address,
      final Bytes32 key,
      final Bytes32 current,
      final Bytes32 next) {
    return new TransientFragment(standardDomSubStamps(hubStamp, 0), address, key, current, next);
  }

  public static TransientFragment tstoreUndoing(
      final int hubStamp, final int revertStamp, final TransientFragment doingFragment) {
    return new TransientFragment(
        revertWithCurrentDomSubStamps(hubStamp, revertStamp, 0),
        doingFragment.address,
        doingFragment.key,
        doingFragment.valueNext,
        doingFragment.valueCurr);
  }

  @Override
  public Trace.Hub trace(Trace.Hub trace) {
    domSubStampsSubFragment.traceHub(trace);

    return trace
        .peekAtTransient(true)
        .pTransientAddressHi(bytesToLong(address.getBytes().slice(0, 4)))
        .pTransientAddressLo(address.getBytes().slice(4, LLARGE))
        .pTransientStorageKeyHi(key.slice(0, LLARGE))
        .pTransientStorageKeyLo(key.slice(LLARGE, LLARGE))
        .pTransientValueCurrHi(valueCurr.slice(0, LLARGE))
        .pTransientValueCurrLo(valueCurr.slice(LLARGE, LLARGE))
        .pTransientValueNextHi(valueNext.slice(0, LLARGE))
        .pTransientValueNextLo(valueNext.slice(LLARGE, LLARGE));
  }
}
