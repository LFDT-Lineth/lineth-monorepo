import { createLogger, format, transports, Logger } from "winston";

const { splat, combine, colorize, timestamp, printf } = format;

export const baseLogger: Logger = createLogger({
  level: process.env.LOG_LEVEL || "info",
  format: combine(
    timestamp(),
    colorize({ level: true }),
    splat(),
    printf(({ level, message, timestamp, testName }) => {
      const testContext = testName ? ` test=${testName}` : "";
      return `timestamp=${timestamp} level=${level}${testContext} | message=${message}`;
    }),
  ),
  transports: [new transports.Console()],
});
