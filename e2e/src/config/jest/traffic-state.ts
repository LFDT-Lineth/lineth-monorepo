type StopTrafficFn = () => Promise<void>;

declare global {
  var __stopL2TrafficFn: StopTrafficFn | null;
}

export function setStopL2TrafficGeneration(fn: StopTrafficFn) {
  globalThis.__stopL2TrafficFn = fn;
}

export async function stopL2TrafficGeneration(): Promise<void> {
  const stopFn = globalThis.__stopL2TrafficFn;
  if (stopFn) {
    await stopFn();
    globalThis.__stopL2TrafficFn = null;
  }
}
