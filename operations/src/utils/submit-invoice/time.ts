import { startOfDay, addDays, isBefore, toDate, addSeconds } from "date-fns";
import { fromZonedTime, toZonedTime } from "date-fns-tz";

function startOfUTCDay(timestamp: number): Date {
  const utcDate = toZonedTime(timestamp, "UTC");
  const midnight = startOfDay(utcDate);
  return fromZonedTime(midnight, "UTC");
}

export type InvoicePeriod = {
  startDate: Date;
  endDate: Date;
};

/**
 * Compute the start and end date of the invoice period.
 * @param lastInvoiceDateInSeconds Unix seconds, UTC timestamp of the last invoice date
 * @param currentTimestampInSeconds Unix seconds, current timestamp
 * @param numberOfInvoicingDays Number of days for the invoice period
 * @returns Start and end date of the invoice period or null if the end date is in the future
 */
export function computeInvoicePeriod(
  lastInvoiceTimestampInSeconds: number,
  currentTimestampInSeconds: number,
  numberOfInvoicingDays: number,
  reportingLagDays: number,
): InvoicePeriod | null {
  const lastInvoiceDate = toDate(lastInvoiceTimestampInSeconds * 1000);
  const currentMidnight = startOfUTCDay(currentTimestampInSeconds * 1000);

  const startDate = addSeconds(lastInvoiceDate, 1);
  const endDate = addDays(lastInvoiceDate, numberOfInvoicingDays);

  if (isBefore(currentMidnight, addDays(endDate, reportingLagDays))) {
    return null;
  }

  return { startDate, endDate };
}
