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

package net.consensys.linea.zktracer.module.txndata.london;

import static net.consensys.linea.zktracer.Trace.RLP_TXN_PHASE_S;

import lombok.Builder;
import org.apache.tuweni.bytes.Bytes;

@Builder
public record RlptxnOutgoing(int phase, Bytes outGoingHi, Bytes outGoingLo) {
  public static RlptxnOutgoing set(
      final short phase, final Bytes outGoingHi, final Bytes outGoingLo) {
    return RlptxnOutgoing.builder()
        .phase(phase)
        .outGoingHi(outGoingHi)
        .outGoingLo(outGoingLo)
        .build();
  }

  public static RlptxnOutgoing empty() {
    return RlptxnOutgoing.builder()
        /* just to not break the lookup, it could be RLP_TXN_PHASE_R, only requirement is
        - a phase that is always present (so not RLP_TXN_PHASE_Y / RLP_TXN_PHASE_BETA);
        - without anything written in DATA_HI / DATA_LO (so not all other phases).
        We could either add a selector in the lookup. */
        .phase(RLP_TXN_PHASE_S)
        .outGoingHi(Bytes.EMPTY)
        .outGoingLo(Bytes.EMPTY)
        .build();
  }
}
