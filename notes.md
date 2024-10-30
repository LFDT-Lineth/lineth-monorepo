# Notes

Balance transfers associated with CALL's and CREATE's are done in the `start` method as follows:
```java
  public void process(final MessageFrame frame, final OperationTracer operationTracer) {
    if (operationTracer != null) {
      if (frame.getState() == MessageFrame.State.NOT_STARTED) {
        operationTracer.traceContextEnter(frame);
        start(frame, operationTracer);
      }
```
See in [Besu](https://github.com/hyperledger/besu/blob/22a570eda42dd4bba917bcaac93bdf642fe765fd/evm/src/main/java/org/hyperledger/besu/evm/processor/AbstractMessageProcessor.java#L190).

The main point being that `traceContextEnter` happens before. However, for some reason the warming of the createe has already taken place.

The same pattern seems to be at play when invoking `traceContextExit`:
```java
  public void process(final MessageFrame frame, final OperationTracer operationTracer) {

    // ...

    if (frame.getState() == MessageFrame.State.COMPLETED_SUCCESS) {
      if (operationTracer != null) {
        operationTracer.traceContextExit(frame);
      }
      completedSuccess(frame);
    }
    if (frame.getState() == MessageFrame.State.COMPLETED_FAILED) {
      if (operationTracer != null) {
        operationTracer.traceContextExit(frame);
      }
      completedFailed(frame);
    }
```
