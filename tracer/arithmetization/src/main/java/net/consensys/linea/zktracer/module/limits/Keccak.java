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

package net.consensys.linea.zktracer.module.limits;

import static com.google.common.base.Preconditions.checkState;
import static net.consensys.linea.zktracer.module.ModuleName.BLOCK_KECCAK;

import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.container.module.CountingOnlyModule;
import net.consensys.linea.zktracer.container.module.IncrementingModule;

@Getter
@Accessors(fluent = true)
public class Keccak extends CountingOnlyModule {
  private static final int PUBKEY_BYTES = 64;
  private static final int KECCAK_BIT_RATE = 1088;
  private static final int KECCAK_BYTE_RATE = KECCAK_BIT_RATE / 8;

  private final IncrementingModule ecRecoverEffectiveCall;
  private final BlockTransactions blockTransactions;

  public Keccak(IncrementingModule ecRecoverEffectiveCall, BlockTransactions blockTransactions) {
    super(BLOCK_KECCAK);
    this.ecRecoverEffectiveCall = ecRecoverEffectiveCall;
    this.blockTransactions = blockTransactions;
  }

  @Override
  public void updateTally(final int count) {
    final int blockCount = numberOfKeccakBloc(count);
    counts.add(blockCount);
  }

  @Override
  public int lineCount() {
    final int txCount = blockTransactions.lineCount();
    final int ecRecoverCount = ecRecoverEffectiveCall.lineCount();

    return
    // Counts:
    // - From tx RLPs, used both for both the signature verification and the public input
    // computation.
    // - From deployed contracts, number of Keccak block for SHA3 and CREATE2, and from RLP_ADDR
    counts.lineCount()
        // From ecRecover precompiles,
        // This accounts for the keccak of the recovered public keys to derive the
        // addresses. This also accounts for the transactions signatures
        // verifications.
        + (txCount + ecRecoverCount) * numberOfKeccakBloc(PUBKEY_BYTES);
  }

  public static int numberOfKeccakBloc(final long dataByteLength) {
    final long r = (dataByteLength + KECCAK_BYTE_RATE - 1) / KECCAK_BYTE_RATE;
    checkState(r < Integer.MAX_VALUE, "demented KECCAK");
    return (int) r;
  }
}
