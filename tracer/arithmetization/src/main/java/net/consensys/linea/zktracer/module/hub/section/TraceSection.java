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

import static com.google.common.base.Preconditions.*;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_EXEC;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_FINL;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_INIT;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_SKIP;
import static net.consensys.linea.zktracer.module.hub.HubProcessingPhase.TX_WARM;

import java.util.ArrayList;
import java.util.List;
import lombok.Getter;
import lombok.Setter;
import lombok.experimental.Accessors;
import net.consensys.linea.zktracer.Trace;
import net.consensys.linea.zktracer.module.hub.Hub;
import net.consensys.linea.zktracer.module.hub.HubProcessingPhase;
import net.consensys.linea.zktracer.module.hub.fragment.ContextFragment;
import net.consensys.linea.zktracer.module.hub.fragment.StackFragment;
import net.consensys.linea.zktracer.module.hub.fragment.TraceFragment;
import net.consensys.linea.zktracer.module.hub.fragment.common.CommonFragment;
import net.consensys.linea.zktracer.module.hub.fragment.common.CommonFragmentValues;
import net.consensys.linea.zktracer.runtime.callstack.CallFrame;
import net.consensys.linea.zktracer.runtime.stack.Stack;
import net.consensys.linea.zktracer.runtime.stack.StackLine;
import org.apache.tuweni.bytes.Bytes;

@Accessors(fluent = true)
/* A TraceSection gather the trace lines linked to a single operation */
public class TraceSection {
  public final CommonFragmentValues commonValues;
  @Getter List<TraceFragment> fragments;
  /* A link to the previous section */
  @Setter public TraceSection previousSection = null;
  /* A link to the next section */
  @Setter public TraceSection nextSection = null;
  // Maybe added to the fragments at end transaction after all defers are resolved
  @Setter public ContextFragment exceptionalContextFragment = null;

  /** Default creator specifying the max number of rows the section can contain. */
  public TraceSection(final Hub hub, final short maxNumberOfLines) {
    hub.state().stamps().incrementHubStamp();
    commonValues = new CommonFragmentValues(hub);
    fragments = new ArrayList<>(maxNumberOfLines);
    hub.addTraceSection(this);
  }

  /**
   * Add a fragment to the section.
   *
   * @param fragment the fragment to insert
   */
  public final void addFragment(TraceFragment fragment) {
    checkArgument(
        !(fragment instanceof CommonFragment),
        "CommonFragment should not be added directly to a trace section");
    fragments.add(fragment);
  }

  /**
   * Add the fragments containing the stack lines.
   *
   * @param hub the execution context
   */
  public final void addStack(Hub hub) {
    for (var stackFragment : this.makeStackFragments(hub)) {
      this.addFragment(stackFragment);
    }
  }

  /**
   * Add several fragments within this section for the specified fragments.
   *
   * @param fragments the fragments to add to the section
   */
  public final void addFragments(TraceFragment... fragments) {
    for (TraceFragment f : fragments) {
      this.addFragment(f);
    }
  }

  /**
   * Add several fragments within this section for the specified fragments.
   *
   * @param fragments the fragments to add to the section
   */
  public final void addFragments(List<TraceFragment> fragments) {
    for (TraceFragment f : fragments) {
      this.addFragment(f);
    }
  }

  /**
   * Insert Stack fragments related to the current state of the stack, then insert the provided
   * fragments in a single swoop.
   *
   * @param hub the execution context
   * @param fragments the fragments to insert
   */
  public final void addStackAndFragments(Hub hub, TraceFragment... fragments) {
    this.addStack(hub);
    this.addFragments(fragments);
  }

  /** This method is called at commit time, to build required information post-hoc. */
  public void seal() {
    final HubProcessingPhase currentPhase = commonValues.hubProcessingPhase;
    final int nsr =
        commonValues.hubProcessingPhase == TX_EXEC
            ? (int) fragments.stream().filter(l -> !(l instanceof StackFragment)).count()
            : 0;
    commonValues.numberOfNonStackRows(nsr);
    commonValues.TLI(
        (int) fragments.stream().filter(l -> (l instanceof StackFragment)).count() == 2);
    commonValues.codeFragmentIndex(
        currentPhase == TX_EXEC
            ? hub()
                .getCodeFragmentIndexByMetaData(
                    commonValues.callFrame().byteCodeAddress(),
                    commonValues.callFrame().byteCodeDeploymentNumber(),
                    commonValues.callFrame().isDeployment(),
                    commonValues.callFrame().delegationNumber())
            : 0);
    commonValues.contextNumberNew(computeContextNumberNew());

    commonValues.gasRefund(
        currentPhase == TX_SKIP || currentPhase == TX_WARM || currentPhase == TX_INIT
            ? 0
            : previousSection.commonValues.gasRefundNew);
    commonValues.gasRefundNew(commonValues.gasRefund + commonValues.refundDelta);

    /* If the logStamp hasn't been set (either by being first section of the tx, or by the LogSection), set it to the previous section logStamp */
    if (commonValues.logStamp == -1) {
      commonValues.logStamp(previousSection.commonValues.logStamp);
    }
  }

  private int computeContextNumberNew() {
    final HubProcessingPhase currentPhase = commonValues.hubProcessingPhase;
    if (currentPhase == TX_WARM || currentPhase == TX_FINL || currentPhase == TX_SKIP) {
      return 0;
    }

    if (nextSection == null) {
      throw new RuntimeException(
          "NPE: nextSection is "
              + nextSection
              + ", current section is of type "
              + this.getClass().getTypeName());
    }
    return nextSection.commonValues.hubProcessingPhase == TX_EXEC
        ? nextSection.commonValues.callFrame().contextNumber()
        : 0;
  }

  private List<TraceFragment> makeStackFragments(final Hub hub) {
    final CallFrame currentFrame = hub.currentFrame();
    final List<TraceFragment> stackFragments = new ArrayList<>(2);
    final Stack snapshot = currentFrame.stack().snapshot();
    if (currentFrame.pending().lines().isEmpty()) {
      for (int i = 0; i < (hub.opCodeData().numberOfStackRows()); i++) {
        stackFragments.add(
            StackFragment.prepare(
                hub,
                snapshot,
                new StackLine().asStackItems(),
                hub.pch().exceptions(),
                hub.pch().abortingConditions().snapshot(),
                hub.gasProjector.of(currentFrame.frame(), hub.opCodeData()),
                currentFrame.isDeployment(),
                commonValues));
      }
    } else {
      for (StackLine line : currentFrame.pending().lines()) {
        stackFragments.add(
            StackFragment.prepare(
                hub,
                snapshot,
                line.asStackItems(),
                hub.pch().exceptions(),
                hub.pch().abortingConditions().snapshot(),
                hub.gasProjector.of(currentFrame.frame(), hub.opCodeData()),
                currentFrame.isDeployment(),
                commonValues));
      }
    }
    return stackFragments;
  }

  public void writeHashInfoResult(Bytes hash) {
    for (TraceFragment fragment : this.fragments()) {
      if (fragment instanceof StackFragment) {
        ((StackFragment) fragment).hash = hash;
      }
    }
  }

  public void triggerJumpDestinationVetting() {
    for (TraceFragment fragment : this.fragments()) {
      if (fragment instanceof StackFragment) {
        ((StackFragment) fragment).jumpDestinationVettingRequired(true);
      }
    }
  }

  public void trace(Trace.Hub hubTrace) {
    int stackLineCounter = -1;
    int nonStackLineCounter = 0;

    for (TraceFragment specificFragment : fragments()) {
      if (specificFragment instanceof StackFragment) {
        stackLineCounter++;
      } else {
        if (commonValues.hubProcessingPhase == TX_EXEC) {
          nonStackLineCounter++;
        }
      }

      specificFragment.trace(hubTrace);
      final CommonFragment commonFragment =
          new CommonFragment(
              commonValues,
              stackLineCounter,
              nonStackLineCounter,
              hub().state.mmuStamp(),
              hub().state.mxpStamp());
      commonFragment.trace(hubTrace);
      hubTrace.fillAndValidateRow();
    }
  }

  public int hubStamp() {
    return commonValues.hubStamp;
  }

  public int revertStamp() {
    return commonValues.callFrame().revertStamp();
  }

  protected Hub hub() {
    return commonValues.hub;
  }
}
