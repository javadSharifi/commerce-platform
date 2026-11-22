import React from 'react';
import {
  Select as RACSelect,
  Label as RACLabel,
  Button as RACButton,
  SelectValue as RACSelectValue,
  Popover as RACPopover,
  ListBox as RACListBox,
  ListBoxItem as RACListBoxItem,
  ListBoxSection as RACListBoxSection,
  Header as RACHeader,
  type SelectProps as RACSelectProps,
} from 'react-aria-components';
import { cn } from '../utils/cn.js';

export interface SelectOption {
  id: string | number;
  label: React.ReactNode;
  disabled?: boolean;
}

export interface SelectProps<T extends object = SelectOption>
  extends Omit<RACSelectProps<T>, 'children'> {
  label?: React.ReactNode;
  description?: React.ReactNode;
  errorMessage?: React.ReactNode;
  placeholder?: string;
  size?: 'sm' | 'md' | 'lg';
  options?: SelectOption[];
  children?: React.ReactNode | ((item: T) => React.ReactNode);
}

const sizeClasses: Record<'sm' | 'md' | 'lg', string> = {
  sm: 'h-8 py-1.5 px-2.5 text-xs',
  md: 'h-10 py-2 px-3 text-sm',
  lg: 'h-12 py-2.5 px-4 text-base',
};

/**
 * Accessible Select component built on React Aria and styled with DaisyUI tokens.
 * Fully supports RTL and SSR with native fallback.
 */
export const Select = <T extends object = SelectOption>({
  label,
  description,
  errorMessage,
  placeholder = 'انتخاب کنید...',
  size = 'md',
  options,
  children,
  className,
  ...props
}: SelectProps<T>) => {
  return (
    <RACSelect
      className={cn('flex flex-col gap-1.5 w-full text-start', className)}
      {...props}
    >
      {label && (
        <RACLabel className="label-text text-sm font-medium text-base-content text-start">
          {label}
        </RACLabel>
      )}

      <RACButton
        className={cn(
          'flex items-center justify-between w-full rounded-box border border-base-300 bg-base-100 text-base-content shadow-xs transition-colors',
          'hover:border-base-content/30 focus:outline-none focus:ring-2 focus:ring-primary/20 focus:border-primary',
          'disabled:opacity-50 disabled:cursor-not-allowed cursor-pointer text-start',
          sizeClasses[size]
        )}
      >
        <RACSelectValue className="flex-1 text-start truncate font-normal">
          {({ defaultChildren, isPlaceholder }) =>
            isPlaceholder ? (
              <span className="text-base-content/50">{placeholder}</span>
            ) : (
              defaultChildren
            )
          }
        </RACSelectValue>
        <span aria-hidden="true" className="ms-2 text-xs opacity-60 shrink-0">
          ▼
        </span>
      </RACButton>

      {description && (
        <span className="text-xs text-base-content/70">{description}</span>
      )}
      {errorMessage && (
        <span className="text-xs text-error font-medium">{errorMessage}</span>
      )}

      <RACPopover
        className={cn(
          'z-50 min-w-(--trigger-width) bg-base-100 text-base-content rounded-box border border-base-200 shadow-xl p-1 max-h-60 overflow-y-auto',
          'animate-in fade-in-0 zoom-in-95 duration-100'
        )}
      >
        <RACListBox className="outline-none flex flex-col gap-0.5 p-0.5">
          {options
            ? options.map((opt) => (
                <RACListBoxItem
                  key={opt.id}
                  id={opt.id}
                  textValue={typeof opt.label === 'string' ? opt.label : String(opt.id)}
                  isDisabled={opt.disabled}
                  className={({ isSelected, isFocused, isDisabled }) =>
                    cn(
                      'flex items-center px-3 py-2 text-sm rounded-btn cursor-pointer transition-colors outline-none text-start',
                      isFocused && 'bg-base-200',
                      isSelected && 'bg-primary text-primary-content font-medium',
                      isDisabled && 'opacity-40 cursor-not-allowed'
                    )
                  }
                >
                  {opt.label}
                </RACListBoxItem>
              ))
            : children}
        </RACListBox>
      </RACPopover>
    </RACSelect>
  );
};

// Re-exports for advanced headless composition
export {
  RACSelect as SelectRoot,
  RACLabel as SelectLabel,
  RACButton as SelectButton,
  RACSelectValue as SelectValue,
  RACPopover as SelectPopover,
  RACListBox as SelectListBox,
  RACListBoxItem as SelectItem,
  RACListBoxSection as SelectSection,
  RACHeader as SelectHeader,
};
