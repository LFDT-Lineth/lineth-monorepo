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
package net.consensys.linea.zktracer.module.hub.fragment.scenario;

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.fragment.scenario.CallScenarioFragment.CallScenario.*;

import lombok.Getter;
import lombok.Setter;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;

public class CallScenarioFragment implements TraceFragment {

  @Setter @Getter CallScenario scenario;

  public CallScenarioFragment() {
    scenario = UNDEFINED;
  }

  public enum CallScenario {
    UNDEFINED,
    CALL_EXCEPTION,
    CALL_ABORT_WILL_REVERT,
    CALL_ABORT_WONT_REVERT,
    // Externally owned account call scenarios
    CALL_EOA_UNDEFINED,
    CALL_EOA_SUCCESS_WILL_REVERT,
    CALL_EOA_SUCCESS_WONT_REVERT,
    // Smart contract call scenarios:
    CALL_SMC_UNDEFINED,
    CALL_SMC_FAILURE_WILL_REVERT,
    CALL_SMC_FAILURE_WONT_REVERT,
    CALL_SMC_SUCCESS_WILL_REVERT,
    CALL_SMC_SUCCESS_WONT_REVERT,
    // Precompile call scenarios:
    CALL_PRC_UNDEFINED,
    CALL_PRC_FAILURE,
    CALL_PRC_SUCCESS_WILL_REVERT,
    CALL_PRC_SUCCESS_WONT_REVERT;

    public boolean isAnyOf(CallScenario... scenarios) {
      for (CallScenario scenario : scenarios) {
        if (this == scenario) {
          return true;
        }
      }
      return false;
    }

    public boolean isIndefinitePrcCallScenario() {
      return this == CALL_PRC_UNDEFINED || isPrcCallScenario();
    }

    public boolean isPrcCallScenario() {
      return this.isAnyOf(
          CALL_PRC_FAILURE, CALL_PRC_SUCCESS_WILL_REVERT, CALL_PRC_SUCCESS_WONT_REVERT);
    }

    public boolean isIndefiniteSmcCallScenario() {
      return this == CALL_SMC_UNDEFINED || isSmcCallScenario();
    }

    public boolean isSmcCallScenario() {
      return this.isAnyOf(
          CALL_SMC_FAILURE_WILL_REVERT,
          CALL_SMC_FAILURE_WONT_REVERT,
          CALL_SMC_SUCCESS_WILL_REVERT,
          CALL_SMC_SUCCESS_WONT_REVERT);
    }

    public boolean isIndefiniteEoaCallScenario() {
      return this == CALL_EOA_UNDEFINED || isEoaCallScenario();
    }

    public boolean isEoaCallScenario() {
      return this.isAnyOf(CALL_EOA_SUCCESS_WILL_REVERT, CALL_EOA_SUCCESS_WONT_REVERT);
    }

    public boolean isAbortingScenario() {
      return this.isAnyOf(CALL_ABORT_WILL_REVERT, CALL_ABORT_WONT_REVERT);
    }

    public boolean noLongerUndefined() {
      return !this.isAnyOf(UNDEFINED, CALL_PRC_UNDEFINED, CALL_SMC_UNDEFINED, CALL_EOA_UNDEFINED);
    }
  }

  public Trace.Hub trace(Trace.Hub trace) {
    checkArgument(
        scenario.noLongerUndefined(),
        "Final " + CallScenario.class.getSimpleName() + ", " + scenario + ", is still undefined");
    return trace
        .peekAtScenario(true)
        // // CALL scenarios
        ////////////////////
        .pScenarioCallException(scenario.equals(CALL_EXCEPTION))
        .pScenarioCallAbortWillRevert(scenario.equals(CALL_ABORT_WILL_REVERT))
        .pScenarioCallAbortWontRevert(scenario.equals(CALL_ABORT_WONT_REVERT))
        .pScenarioCallPrcFailure(scenario.equals(CALL_PRC_FAILURE))
        .pScenarioCallPrcSuccessCallerWillRevert(scenario.equals(CALL_PRC_SUCCESS_WILL_REVERT))
        .pScenarioCallPrcSuccessCallerWontRevert(scenario.equals(CALL_PRC_SUCCESS_WONT_REVERT))
        .pScenarioCallSmcFailureCallerWillRevert(scenario.equals(CALL_SMC_FAILURE_WILL_REVERT))
        .pScenarioCallSmcFailureCallerWontRevert(scenario.equals(CALL_SMC_FAILURE_WONT_REVERT))
        .pScenarioCallSmcSuccessCallerWillRevert(scenario.equals(CALL_SMC_SUCCESS_WILL_REVERT))
        .pScenarioCallSmcSuccessCallerWontRevert(scenario.equals(CALL_SMC_SUCCESS_WONT_REVERT))
        .pScenarioCallEoaSuccessCallerWillRevert(scenario.equals(CALL_EOA_SUCCESS_WILL_REVERT))
        .pScenarioCallEoaSuccessCallerWontRevert(scenario.equals(CALL_EOA_SUCCESS_WONT_REVERT));
  }
}
