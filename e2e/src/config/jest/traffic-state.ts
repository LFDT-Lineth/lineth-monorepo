type StopTrafficFn = () => Promise<void>;

let stopTrafficFn: StopTrafficFn | null = null;

export function setStopL2TrafficGeneration(fn: StopTrafficFn) {
  stopTrafficFn = fn;
}

export async function stopL2TrafficGeneration(): Promise<void> {
  if (stopTrafficFn) {
    await stopTrafficFn();
    stopTrafficFn = null;
  }
}
