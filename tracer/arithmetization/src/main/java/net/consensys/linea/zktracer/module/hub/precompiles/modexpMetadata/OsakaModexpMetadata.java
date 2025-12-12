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
package net.consensys.linea.zktracer.module.hub.precompiles.modexpMetadata;

import static net.consensys.linea.zktracer.TraceOsaka.EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND;
import static net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase.*;

import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase;
import net.consensys.linea.zktracer.types.MemoryRange;
import org.apache.tuweni.bytes.Bytes;
import org.hyperledger.besu.evm.internal.Words;

public class OsakaModexpMetadata extends LondonModexpMetadata {
  public OsakaModexpMetadata(MemoryRange callDataRange) {
    super(callDataRange);
  }

  /** Linea supports the full breadth of Osaka-EVM-legal MODEXP inputs. */
  @Override
  public boolean unprovableModexp() {
    return false;
  }

  @Override
  public boolean tracedIsWithinBounds(ModexpXbsCase modexpXbsCase) {
    return (Words.clampedToInt(xbs(modexpXbsCase)) <= getMaxInputSize());
  }

  @Override
  public boolean tracedIsOutOfBounds(ModexpXbsCase modexpXbsCase) {
    return !tracedIsWithinBounds(modexpXbsCase);
  }

  @Override
  public int getMaxInputSize() {
    return EIP_7823_MODEXP_UPPER_BYTE_SIZE_BOUND;
  }

  @Override
  public Bytes normalize(ModexpXbsCase modexpXbsCase) {
    return tracedIsWithinBounds(modexpXbsCase) ? xbs(modexpXbsCase).toBytes() : Bytes.EMPTY;
  }

  @Override
  public int getLeadLogByteMultiplier() {
    return 16;
  }

  @Override
  public boolean loadRawLeadingWord() {
    return callData().size() > BASE_MIN_OFFSET + normalizedBbsInt() && normalizedEbsInt() != 0;
  }

  @Override
  public boolean allXbsesAreInBounds() {
    return tracedIsWithinBounds(MODEXP_XBS_CASE_BBS)
        && tracedIsWithinBounds(MODEXP_XBS_CASE_EBS)
        && tracedIsWithinBounds(MODEXP_XBS_CASE_MBS);
  }
}
