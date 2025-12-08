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

import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase;
import net.consensys.linea.zktracer.types.MemoryRange;
import org.apache.tuweni.bytes.Bytes;

public class LondonModexpMetadata extends ModexpMetadata {

  public LondonModexpMetadata(MemoryRange callDataRange) {
    super(callDataRange);
  }

  @Override
  public boolean unprovableModexp() {
    return bbs().toUnsignedBigInteger().compareTo(getMaxInputSizeBigInteger()) > 0
        || mbs().toUnsignedBigInteger().compareTo(getMaxInputSizeBigInteger()) > 0
        || ebs().toUnsignedBigInteger().compareTo(getMaxInputSizeBigInteger()) > 0;
  }

  @Override
  public int getLeadLogByteMultiplier() {
    return 8;
  }

  @Override
  public int getMaxInputSize() {
    return 512;
  }

  @Override
  public Bytes normalize(ModexpXbsCase modexpXbsCase) {
    return xbs(modexpXbsCase).lo();
  }

  @Override
  public boolean loadRawLeadingWord() {
    return callData().size() > BASE_MIN_OFFSET + bbsInt() && !ebs().isZero();
  }

  @Override
  public boolean allXbsesAreInBounds() {
    return true;
  }
}
