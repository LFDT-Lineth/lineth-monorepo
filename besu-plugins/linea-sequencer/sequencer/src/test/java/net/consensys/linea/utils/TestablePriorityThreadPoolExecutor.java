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
package net.consensys.linea.utils;

import static org.awaitility.Awaitility.await;

import java.util.concurrent.Callable;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Semaphore;
import java.util.concurrent.ThreadFactory;
import java.util.concurrent.TimeUnit;

import lombok.RequiredArgsConstructor;
import org.hyperledger.besu.util.Subscribers;

public class TestablePriorityThreadPoolExecutor extends PriorityThreadPoolExecutor {
  public interface BeforeExecuteListener {
    void onBeforeExecute(Thread t, Runnable r);
  }

  public interface AfterExecuteListener {
    void onAfterExecute(Runnable r, final Throwable t);
  }

  private final Subscribers<BeforeExecuteListener> beforeExecuteListeners = Subscribers.create();
  private final Subscribers<AfterExecuteListener> afterExecuteListeners = Subscribers.create();
  private final Subscribers<BeforeExecuteListener> internalBeforeExecuteListeners =
      Subscribers.create();
  private final Subscribers<AfterExecuteListener> internalAfterExecuteListeners =
      Subscribers.create();

  public TestablePriorityThreadPoolExecutor(
      final int corePoolSize,
      final int maximumPoolSize,
      final long keepAliveTime,
      final TimeUnit unit,
      final ThreadFactory threadFactory) {
    super(corePoolSize, maximumPoolSize, keepAliveTime, unit, threadFactory);
  }

  @Override
  protected void beforeExecute(final Thread t, final Runnable r) {
    super.beforeExecute(t, r);
    if (isInternalTask(r)) {
      internalBeforeExecuteListeners.forEach(l -> l.onBeforeExecute(t, r));
    } else {
      beforeExecuteListeners.forEach(l -> l.onBeforeExecute(t, r));
    }
  }

  @Override
  protected void afterExecute(final Runnable r, final Throwable t) {
    super.afterExecute(r, t);
    if (isInternalTask(r)) {
      internalAfterExecuteListeners.forEach(l -> l.onAfterExecute(r, t));
    } else {
      afterExecuteListeners.forEach(l -> l.onAfterExecute(r, t));
    }
  }

  public long addBeforeExecuteListener(final BeforeExecuteListener l) {
    return beforeExecuteListeners.subscribe(l);
  }

  public long addAfterExecuteListener(final AfterExecuteListener l) {
    return afterExecuteListeners.subscribe(l);
  }

  public void removeBeforeExecuteListeners(final long id) {
    beforeExecuteListeners.unsubscribe(id);
  }

  public void removeAfterExecuteListeners(final long id) {
    afterExecuteListeners.unsubscribe(id);
  }

  public void waitForQueueTaskCount(final int expectedCount, final boolean blocking)
      throws ExecutionException, InterruptedException {
    final var waitTask = new WaitForQueueTaskCount(expectedCount);
    if (blocking) {
      ensureInternalTaskIsSelectedForExecution(waitTask);
    } else {
      submit(waitTask);
    }
  }

  public Semaphore pauseExecution() throws InterruptedException {
    final var resumeSemaphore = new Semaphore(0);
    final var pauseTask = new PauseExecution(resumeSemaphore);
    ensureInternalTaskIsSelectedForExecution(pauseTask);
    return resumeSemaphore;
  }

  public void executeSomething() {
    submit(new FillerTask());
  }

  private void ensureInternalTaskIsSelectedForExecution(final InternalTask task)
      throws InterruptedException {
    final var returnSemaphore = new Semaphore(0);
    final var lid =
        internalBeforeExecuteListeners.subscribe(
            (t, r) -> {
              // wait that this pause task is being executed before returning
              if (extractPriorityFuture(r).getSourceTask() == task) {
                returnSemaphore.release();
              }
            });
    submit(task);
    returnSemaphore.acquire();
    internalBeforeExecuteListeners.unsubscribe(lid);
  }

  @SuppressWarnings("unchecked")
  private PriorityThreadPoolExecutor.PriorityFuture<?> extractPriorityFuture(final Runnable r) {
    return (PriorityThreadPoolExecutor.PriorityFuture<?>) r;
  }

  private boolean isInternalTask(final Runnable r) {
    return extractPriorityFuture(r).getSourceTask() instanceof InternalTask;
  }

  private interface InternalTask extends Callable<Void>, Comparable<Object> {}

  @RequiredArgsConstructor
  private class WaitForQueueTaskCount implements InternalTask {
    private final int expectedCount;

    @Override
    public int compareTo(final Object o) {
      // force the internal task to always have priority
      return -1;
    }

    @Override
    public Void call() {
      await().until(() -> getQueue().size() == expectedCount);
      return null;
    }
  }

  @RequiredArgsConstructor
  private class PauseExecution implements InternalTask {
    private final Semaphore semaphore;

    @Override
    public int compareTo(final Object o) {
      // force the internal task to always have priority
      return -1;
    }

    @Override
    public Void call() throws Exception {
      semaphore.acquire();
      return null;
    }
  }

  private class FillerTask implements InternalTask {

    @Override
    public int compareTo(final Object o) {
      // execute after all other queued tasks
      return 1;
    }

    @Override
    public Void call() {
      return null;
    }
  }
}
