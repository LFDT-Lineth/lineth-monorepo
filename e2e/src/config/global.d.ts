/* eslint-disable no-var */

import { Logger } from "winston";

declare global {
  var stopL2TrafficGeneration: () => void;
  var logger: Logger;
}

export {};
