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

package net.consensys.linea.zktracer.module.mmu.values;

import static net.consensys.linea.zktracer.types.Utils.BYTES16_ZERO;

import lombok.Builder;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import org.apache.tuweni.bytes.Bytes;

@Builder
@Getter
@Setter
@Accessors(fluent = true)
public class MmuToMmioInstruction {
  private int mmioInstruction;
  private short size;
  private long sourceLimbOffset;
  private short sourceByteOffset;
  private long targetLimbOffset;
  private short targetByteOffset;
  @Builder.Default private Bytes limb = BYTES16_ZERO;
  private boolean targetLimbIsTouchedTwice;
}
