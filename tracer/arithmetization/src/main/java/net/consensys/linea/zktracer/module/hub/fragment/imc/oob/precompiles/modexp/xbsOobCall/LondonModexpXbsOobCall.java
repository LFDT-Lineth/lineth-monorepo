/*
 * Copyright ConsenSys AG.
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

package net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.xbsOobCall;

import lombok.EqualsAndHashCode;
import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.module.blake2fmodexpdata.LondonBlakeModexpDataOperation;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.oob.precompiles.modexp.ModexpXbsCase;
import net.consensys.linea.zktracer.module.hub.precompiles.modexpMetadata.LondonModexpMetadata;
import org.hyperledger.besu.evm.frame.MessageFrame;

@Getter
@Setter
@EqualsAndHashCode(onlyExplicitlyIncluded = true, callSuper = true)
public class LondonModexpXbsOobCall extends ModexpXbsOobCall {

  public LondonModexpXbsOobCall(LondonModexpMetadata modexpMetaData, ModexpXbsCase modexpXbsCase) {
    super(modexpMetaData, modexpXbsCase);
  }

  protected LondonModexpMetadata getForkAppropriateModexpMetadata() {
    return (LondonModexpMetadata) this.modexpMetadata;
  }

  public int modexpComponentByteSize() {
    return LondonBlakeModexpDataOperation.modexpComponentByteSize();
  }

  @Override
  public void setInputData(MessageFrame frame, Hub hub) {}

  @Override
  short xbsNormalized() {
    return (short) xbs().toInt();
  }

  @Override
  short ybsReNormalized() {
    return (short) ybsLo().toInt();
  }

  @Override
  short maxXbsYbs() {
    return computeMax() ? (short) Math.max(xbsNormalized(), ybsReNormalized()) : 0;
  }

  @Override
  boolean xbsNormalizedIsNonZero() {
    return xbsNormalized() != 0;
  }

  @Override
  boolean xbsNormalizedIsNonZeroTracedValue() {
    return computeMax() && xbsNormalizedIsNonZero();
  }

  @Override
  protected boolean xbsIsWithinBounds() {
    return false;
  }

  @Override
  protected boolean xbsIsOutOfBounds() {
    return false;
  }

  @Override
  protected boolean computeMax() {
    return switch (modexpXbsCase) {
      case MODEXP_XBS_CASE_BBS, MODEXP_XBS_CASE_EBS -> false;
      case MODEXP_XBS_CASE_MBS -> true;
    };
  }
}
