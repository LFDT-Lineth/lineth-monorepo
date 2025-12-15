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

import lombok.Builder;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;

@Builder
@Getter
@Accessors(fluent = true)
public class MmuToMmioConstantValues {

  private final long sourceContextNumber;
  private final long targetContextNumber;
  private final boolean successBit;
  private final int exoSum;
  private final boolean exoIsRom;
  private final boolean exoIsBlake2fModexp;
  private final boolean exoIsEcData;
  private final boolean exoIsRipSha;
  private final boolean exoIsKeccak;
  private final boolean exoIsLog;
  private final boolean exoIsTxcd;
  private final int phase;
  @Setter private int exoId;
  private final int kecId;
  private final long totalSize;
}
