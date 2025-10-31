import { describe, it, expect } from "@jest/globals";
import { computeInvoicePeriod } from "../time.js";
import { fromZonedTime } from "date-fns-tz";

describe("Time", () => {
  describe("computeInvoiceDatesWithDelay", () => {
    it("should return null if the invoice for the previous day has already been submitted", () => {
      const lastInvoiceDateInSecondsUTC =
        fromZonedTime(Math.floor(new Date("2025-10-14 23:59:59").getTime()), "UTC").getTime() / 1000;
      const currentDateInSeconds = new Date("2025-10-15 15:30:00").getTime() / 1000;
      const periodInDays = 2;
      const reportingLagDays = 0;

      expect(
        computeInvoicePeriod(lastInvoiceDateInSecondsUTC, currentDateInSeconds, periodInDays, reportingLagDays),
      ).toBeNull();
    });

    it("should return 1 day invoice period when periodInDays = 1", () => {
      const lastInvoiceDateInSecondsUTC =
        fromZonedTime(Math.floor(new Date("2025-10-14 23:59:59").getTime()), "UTC").getTime() / 1000;
      const currentDateInSeconds = new Date("2025-10-16 15:30:00").getTime() / 1000;
      const periodInDays = 1;
      const reportingLagDays = 0;

      expect(
        computeInvoicePeriod(lastInvoiceDateInSecondsUTC, currentDateInSeconds, periodInDays, reportingLagDays),
      ).toEqual({
        startDate: new Date("2025-10-15T00:00:00.000Z"),
        endDate: new Date("2025-10-15T23:59:59.000Z"),
      });
    });

    it("should return null when currentDate < endDate + reportingLagDays", () => {
      const lastInvoiceDateInSecondsUTC =
        fromZonedTime(Math.floor(new Date("2025-10-14 23:59:59").getTime()), "UTC").getTime() / 1000;
      const currentDateInSeconds = new Date("2025-10-16 15:30:00").getTime() / 1000;
      const periodInDays = 1;
      const reportingLagDays = 1;

      expect(
        computeInvoicePeriod(lastInvoiceDateInSecondsUTC, currentDateInSeconds, periodInDays, reportingLagDays),
      ).toBeNull();
    });

    it("should return period when currentDate >= endDate + reportingLagDays", () => {
      const lastInvoiceDateInSecondsUTC =
        fromZonedTime(Math.floor(new Date("2025-10-13 23:59:59").getTime()), "UTC").getTime() / 1000;
      const currentDateInSeconds = new Date("2025-10-16 15:30:00").getTime() / 1000;
      const periodInDays = 1;
      const reportingLagDays = 1;

      expect(
        computeInvoicePeriod(lastInvoiceDateInSecondsUTC, currentDateInSeconds, periodInDays, reportingLagDays),
      ).toEqual({
        startDate: new Date("2025-10-14T00:00:00.000Z"),
        endDate: new Date("2025-10-14T23:59:59.000Z"),
      });
    });
  });
});
