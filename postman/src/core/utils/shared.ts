/**
 * Subtracts a specified number of seconds from a given date.
 *
 * @param {Date} date - The original date.
 * @param {number} seconds - The number of seconds to subtract from the date.
 * @returns {Date} A new date object representing the time after subtracting the specified seconds.
 */
export const subtractSeconds = (date: Date, seconds: number): Date => {
  const dateCopy = new Date(date);
  dateCopy.setSeconds(date.getSeconds() - seconds);
  return dateCopy;
};
