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
package net.consensys.linea.zktracer.instructionprocessing.createTests;

public enum SizeParameter {
  s_ZERO,
  s_TWELVE, // - 3 - 1
  s_THIRTEEN, // - 3 + 0
  s_FOURTEEN, // - 3 + 1
  s_THIRTY_TWO,
  s_MSIZE,
  s_MAX;

  public boolean isAnyOf(SizeParameter... sizeParameters) {
    for (SizeParameter sizeParameter : sizeParameters) {
      if (this == sizeParameter) {
        return true;
      }
    }
    return false;
  }

  public boolean willRaiseException() {
    return this.isAnyOf(s_MAX);
  }
}
