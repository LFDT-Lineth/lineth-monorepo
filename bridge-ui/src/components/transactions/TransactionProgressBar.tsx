import { fromUnixTime } from "date-fns";
import { OnChainMessageStatus } from "@consensys/linea-sdk";
import { NetworkLayer } from "@/config";
import { cn } from "@/utils/cn";

type TransactionProgressBarProps = {
  status: OnChainMessageStatus;
  transactionTimestamp: bigint;
  fromChain?: NetworkLayer;
};

type TimeUnit = "seconds" | "minutes" | "hours";

const L1_L2_ESTIMATED_TIME_IN_SECONDS = 20 * 60; // 20 minutes
const L2_L1_MIN_TIME_IN_SECONDS = 8 * 60 * 60; // 8 hours
const L2_L1_MAX_TIME_IN_SECONDS = 32 * 60 * 60; // 32 hours

const getElapsedTime = (startTime: Date, currentTime: Date): number => {
  return (currentTime.getTime() - startTime.getTime()) / 1000;
};

const getRemainingTime = (startTime: Date, currentTime: Date, unit: TimeUnit, fromChain: NetworkLayer): string => {
  const totalTime =
    fromChain === NetworkLayer.L1
      ? L1_L2_ESTIMATED_TIME_IN_SECONDS
      : (L2_L1_MIN_TIME_IN_SECONDS + L2_L1_MAX_TIME_IN_SECONDS) / 2;

  const elapsedTime = getElapsedTime(startTime, currentTime);
  const remainingTimeInSeconds = Math.max(totalTime - elapsedTime, 0);

  const hours = Math.floor(remainingTimeInSeconds / 3600);
  const minutes = Math.floor((remainingTimeInSeconds % 3600) / 60);
  const seconds = Math.floor(remainingTimeInSeconds % 60);

  switch (unit) {
    case "seconds":
      return `${seconds} secs`;
    case "minutes":
      return `${minutes} min`;
    case "hours":
      return `${hours} hrs`;
    default:
      throw new Error("Invalid time unit.");
  }
};

const getCompletionPercentage = (
  startTime: Date,
  currentTime: Date,
  status: OnChainMessageStatus,
  fromChain?: NetworkLayer,
): number => {
  if (status === OnChainMessageStatus.CLAIMED) {
    return 100;
  }

  if (!fromChain) {
    throw new Error("Invalid network layer.");
  }

  const elapsedTime = getElapsedTime(startTime, currentTime);

  let totalTime: number;

  if (fromChain === NetworkLayer.L1) {
    totalTime = L1_L2_ESTIMATED_TIME_IN_SECONDS; // Convert 20 minutes to seconds
  } else if (fromChain === NetworkLayer.L2) {
    totalTime = (L2_L1_MIN_TIME_IN_SECONDS + L2_L1_MAX_TIME_IN_SECONDS) / 2;
  } else {
    throw new Error("Invalid network layer.");
  }

  // Calculate the completion percentage
  const completionPercentage = (elapsedTime / totalTime) * 100;

  // Ensure the percentage does not exceed 100%
  return Math.min(completionPercentage, 100);
};

const getProgressBarText = (
  startTime: Date,
  currentTime: Date,
  status: OnChainMessageStatus,
  fromChain?: NetworkLayer,
): string => {
  if (!fromChain) {
    throw new Error("Invalid network layer.");
  }

  if (status === OnChainMessageStatus.CLAIMABLE || status === OnChainMessageStatus.CLAIMED) {
    return "Complete";
  }

  const unit = fromChain === NetworkLayer.L1 ? "minutes" : "hours";

  return `Est time left ${getRemainingTime(startTime, currentTime, unit, fromChain)}`;
};

export default function TransactionProgressBar({
  status,
  transactionTimestamp,
  fromChain,
}: TransactionProgressBarProps) {
  return (
    <>
      <div className="flex items-center gap-2">
        {[OnChainMessageStatus.CLAIMABLE, OnChainMessageStatus.UNKNOWN].includes(status) && (
          <span className="loading loading-spinner loading-xs" />
        )}
        <span className="text-xs uppercase">
          {getProgressBarText(fromUnixTime(Number(transactionTimestamp)), new Date(), status, fromChain)}
        </span>
      </div>
      <progress
        className={cn("progress min-w-fit rounded-none [&::-webkit-progress-value]:rounded-none", {
          "progress-warning": status === OnChainMessageStatus.UNKNOWN,
          "progress-primary": status === OnChainMessageStatus.CLAIMABLE,
          "progress-secondary": status === OnChainMessageStatus.CLAIMED,
        })}
        value={getCompletionPercentage(fromUnixTime(Number(transactionTimestamp)), new Date(), status, fromChain)}
        max={100}
      ></progress>
    </>
  );
}
