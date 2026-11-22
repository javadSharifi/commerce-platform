import React from 'react';
import {
  DatePicker as RACDatePicker,
  Label as RACLabel,
  Group as RACGroup,
  DateInput as RACDateInput,
  DateSegment as RACDateSegment,
  Button as RACButton,
  Popover as RACPopover,
  Dialog as RACDialog,
  Calendar as RACCalendar,
  CalendarGrid as RACCalendarGrid,
  CalendarGridHeader as RACCalendarGridHeader,
  CalendarHeaderCell as RACCalendarHeaderCell,
  CalendarGridBody as RACCalendarGridBody,
  CalendarCell as RACCalendarCell,
  Heading as RACHeading,
  I18nProvider,
  type DatePickerProps as RACDatePickerProps,
  type DateValue,
} from 'react-aria-components';
import { CalendarDate } from '@internationalized/date';
import { cn } from '../utils/cn.js';
import {
  toISODateString,
  parseISODate,
  formatPersianDate,
} from './date-utils.js';

export interface PersianDatePickerProps
  extends Omit<
    RACDatePickerProps<DateValue>,
    'value' | 'defaultValue' | 'onChange' | 'minValue' | 'maxValue' | 'children'
  > {
  label?: React.ReactNode;
  description?: React.ReactNode;
  errorMessage?: React.ReactNode;
  value?: string | DateValue | null;
  defaultValue?: string | DateValue | null;
  onChange?: (isoDate: string, date: CalendarDate | null) => void;
  minValue?: string | DateValue;
  maxValue?: string | DateValue;
  size?: 'sm' | 'md' | 'lg';
  className?: string;
}

const sizeClasses: Record<'sm' | 'md' | 'lg', string> = {
  sm: 'h-8 px-2 text-xs',
  md: 'h-10 px-3 text-sm',
  lg: 'h-12 px-4 text-base',
};

function resolveDateValue(
  val: string | DateValue | null | undefined
): DateValue | null | undefined {
  if (val === undefined) return undefined;
  if (val === null) return null;
  if (typeof val === 'string') {
    return parseISODate(val);
  }
  return val;
}

/**
 * Persian Date Picker supporting the Jalali calendar natively with React Aria Components
 * and @internationalized/date.
 * Displays Persian numerals and months in the UI while emitting standard ISO-8601
 * (YYYY-MM-DD) date strings for API and database operations.
 */
export const PersianDatePicker: React.FC<PersianDatePickerProps> = ({
  label,
  description,
  errorMessage,
  value,
  defaultValue,
  onChange,
  minValue,
  maxValue,
  size = 'md',
  className,
  name,
  ...props
}) => {
  const resolvedValue = resolveDateValue(value);
  const resolvedDefaultValue = resolveDateValue(defaultValue);
  const resolvedMinValue = resolveDateValue(minValue) || undefined;
  const resolvedMaxValue = resolveDateValue(maxValue) || undefined;

  const handleChange = (date: DateValue | null) => {
    const isoString = toISODateString(date);
    onChange?.(isoString, (date as CalendarDate) || null);
  };

  return (
    <I18nProvider locale="fa-IR-u-ca-persian">
      <RACDatePicker
        value={resolvedValue}
        defaultValue={resolvedDefaultValue}
        onChange={handleChange}
        minValue={resolvedMinValue}
        maxValue={resolvedMaxValue}
        name={name}
        className={cn('flex flex-col gap-1.5 w-full text-start', className)}
        {...props}
      >
        {label && (
          <RACLabel className="label-text text-sm font-medium text-base-content text-start">
            {label}
          </RACLabel>
        )}

        <RACGroup
          className={cn(
            'input input-bordered flex items-center justify-between w-full bg-base-100 text-base-content shadow-xs transition-colors',
            'hover:border-base-content/30 focus-within:outline-none focus-within:ring-2 focus-within:ring-primary/20 focus-within:border-primary',
            'disabled:opacity-50 disabled:cursor-not-allowed',
            sizeClasses[size]
          )}
        >
          <RACDateInput className="flex-1 flex items-center gap-1 font-mono text-start outline-none">
            {(segment) => (
              <RACDateSegment
                segment={segment}
                className={cn(
                  'px-0.5 rounded text-center outline-none focus:bg-primary focus:text-primary-content transition-colors',
                  segment.isPlaceholder && 'text-base-content/40'
                )}
              />
            )}
          </RACDateInput>

          <RACButton
            className="btn btn-ghost btn-xs px-2 ms-2 text-base-content/70 hover:text-base-content shrink-0"
            aria-label="باز کردن تقویم"
          >
            <span aria-hidden="true">📅</span>
          </RACButton>
        </RACGroup>

        {description && (
          <span className="text-xs text-base-content/70">{description}</span>
        )}
        {errorMessage && (
          <span className="text-xs text-error font-medium">{errorMessage}</span>
        )}

        <RACPopover
          className={cn(
            'z-50 bg-base-100 text-base-content rounded-box border border-base-200 shadow-2xl p-4',
            'animate-in fade-in-0 zoom-in-95 duration-100'
          )}
        >
          <RACDialog className="outline-none">
            <RACCalendar className="flex flex-col gap-3">
              <header className="flex items-center justify-between mb-2">
                <RACButton
                  slot="previous"
                  className="btn btn-sm btn-ghost btn-circle text-sm"
                  aria-label="ماه قبل"
                >
                  <span aria-hidden="true">‹</span>
                </RACButton>
                <RACHeading className="text-sm font-bold text-base-content" />
                <RACButton
                  slot="next"
                  className="btn btn-sm btn-ghost btn-circle text-sm"
                  aria-label="ماه بعد"
                >
                  <span aria-hidden="true">›</span>
                </RACButton>
              </header>

              <RACCalendarGrid className="border-collapse w-full">
                <RACCalendarGridHeader>
                  {(day) => (
                    <RACCalendarHeaderCell className="text-xs font-semibold text-base-content/60 pb-2 text-center w-8 h-8">
                      {day}
                    </RACCalendarHeaderCell>
                  )}
                </RACCalendarGridHeader>
                <RACCalendarGridBody>
                  {(date) => (
                    <RACCalendarCell
                      date={date}
                      className={({ isSelected, isFocused, isDisabled, isOutsideVisibleRange }) =>
                        cn(
                          'w-8 h-8 flex items-center justify-center rounded-btn text-xs font-medium cursor-pointer transition-colors outline-none text-base-content',
                          'hover:bg-base-200',
                          isSelected && 'bg-primary text-primary-content font-bold hover:bg-primary',
                          isFocused && !isSelected && 'ring-1 ring-primary/40',
                          isOutsideVisibleRange && 'opacity-30',
                          isDisabled && 'opacity-20 cursor-not-allowed'
                        )
                      }
                    />
                  )}
                </RACCalendarGridBody>
              </RACCalendarGrid>
            </RACCalendar>
          </RACDialog>
        </RACPopover>
      </RACDatePicker>
    </I18nProvider>
  );
};
