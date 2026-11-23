import {
  CalendarDate,
  createCalendar,
  DateFormatter,
  parseDate,
  toCalendar,
  toCalendarDate,
  today,
  getLocalTimeZone,
  type DateValue,
} from '@internationalized/date';

/**
 * Converts a CalendarDate or DateValue from any calendar system (Persian, Gregorian, etc.)
 * to standard ISO-8601 date string (YYYY-MM-DD) for database storage and API requests.
 */
export function toISODateString(date: DateValue | null | undefined): string {
  if (!date) return '';
  try {
    const gregDate = toCalendar(date, createCalendar('gregory'));
    return gregDate.toString();
  } catch {
    return '';
  }
}

/**
 * Safely parses an ISO date string (YYYY-MM-DD or ISO timestamp) into a CalendarDate.
 */
export function parseISODate(isoStr: string | null | undefined): CalendarDate | null {
  if (!isoStr) return null;
  try {
    const clean = isoStr.trim().slice(0, 10);
    return parseDate(clean);
  } catch {
    return null;
  }
}

export interface FormatPersianDateOptions {
  style?: 'full' | 'long' | 'medium' | 'short';
}

/**
 * Formats a Gregorian date (string, Date, or CalendarDate) into standard Persian (Jalali) representation.
 * Example: '2026-11-21' -> '۳۰ آبان ۱۴۰۵'
 */
export function formatPersianDate(
  input: string | Date | DateValue | null | undefined,
  options: FormatPersianDateOptions = {}
): string {
  if (!input) return '';
  try {
    let dateObj: Date;
    if (typeof input === 'string') {
      const parsed = parseISODate(input);
      if (!parsed) return '';
      dateObj = parsed.toDate('UTC');
    } else if (input instanceof Date) {
      dateObj = input;
    } else if ('toDate' in input && typeof input.toDate === 'function') {
      dateObj = input.toDate('UTC');
    } else {
      return '';
    }

    const formatter = new DateFormatter('fa-IR-u-ca-persian', {
      dateStyle: options.style || 'long',
      timeZone: typeof input === 'string' || !(input instanceof Date) ? 'UTC' : undefined,
    });
    return formatter.format(dateObj);
  } catch {
    return '';
  }
}

/**
 * Converts an input date to a CalendarDate in the Persian calendar.
 */
export function toPersianCalendarDate(
  input: string | Date | DateValue | null | undefined
): CalendarDate | null {
  if (!input) return null;
  try {
    const cal = createCalendar('persian');
    if (typeof input === 'string') {
      const parsed = parseISODate(input);
      return parsed ? toCalendarDate(toCalendar(parsed, cal)) : null;
    }
    if (input instanceof Date) {
      const year = input.getUTCFullYear();
      const month = input.getUTCMonth() + 1;
      const day = input.getUTCDate();
      const greg = new CalendarDate(createCalendar('gregory'), year, month, day);
      return toCalendarDate(toCalendar(greg, cal));
    }
    if ('calendar' in input) {
      return toCalendarDate(toCalendar(input, cal));
    }
    return null;
  } catch {
    return null;
  }
}

/**
 * Returns today's date in the Persian calendar.
 */
export function getTodayPersian(timeZone: string = getLocalTimeZone()): CalendarDate {
  return toCalendar(today(timeZone), createCalendar('persian'));
}
