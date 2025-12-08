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
package net.consensys.linea.zktracer.instructionprocessing.callTests.prc.ecmul;

/**
 * Call data size parameters for the ECMUL precompile. We provision for either a collection of
 * complete EVM words or a collection of EVM words with the final one either missing one byte or
 * being just 11 bytes.
 */
public enum CallDataSizeParameter {
  EMPTY,
  // partial words
  NONEMPTY_1f, // 32 -  1 bytes
  NONEMPTY_3f, // 64 -  1 bytes
  NONEMPTY_4d, // 64 + 13 bytes
  // full words
  NONEMPTY_20,
  NONEMPTY_40,
  NONEMPTY_60,
  FULL,
  LARGE
}
