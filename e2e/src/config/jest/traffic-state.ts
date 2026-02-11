type StopTrafficFn = () => void;

let stopTrafficFn: StopTrafficFn | null = null;

export function setStopL2TrafficGeneration(fn: StopTrafficFn) {
  stopTrafficFn = fn;
}

export function stopL2TrafficGeneration() {
  if (stopTrafficFn) {
    stopTrafficFn();
    stopTrafficFn = null;
  }
}
