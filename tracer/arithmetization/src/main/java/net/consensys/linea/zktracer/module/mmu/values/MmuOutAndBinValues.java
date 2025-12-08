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

@Builder
public record MmuOutAndBinValues(
    long out1,
    long out2,
    long out3,
    long out4,
    long out5,
    boolean bin1,
    boolean bin2,
    boolean bin3,
    boolean bin4,
    boolean bin5) {
  public static final MmuOutAndBinValues DEFAULT = builder().build();
}
