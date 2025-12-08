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

package net.consensys.linea.zktracer.module.hub;

import lombok.Getter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.module.hub.fragment.account.AccountFragment;

/** Contain factories for modules requiring access to longer-lived data. */
@Accessors(fluent = true)
public class Factories {
  @Getter private final AccountFragment.AccountFragmentFactory accountFragment;

  public Factories(final Hub hub) {
    this.accountFragment = new AccountFragment.AccountFragmentFactory(hub);
  }
}
