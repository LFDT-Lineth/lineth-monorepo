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
package net.consensys.linea.zktracer.module.hub.section;

import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.fragment.imc.ImcFragment;
import net.consensys.linea.zktracer.module.hub.fragment.imc.MxpCall;

public class MsizeSection extends TraceSection {

  public static final short NB_ROWS_HUB_MSIZE = 3;

  public MsizeSection(Hub hub) {
    super(hub, NB_ROWS_HUB_MSIZE);

    final MxpCall mxpCall = MxpCall.newMxpCall(hub);
    final ImcFragment imcFragment = ImcFragment.empty(hub).callMxp(mxpCall);
    this.addStackAndFragments(hub, imcFragment);
  }
}
