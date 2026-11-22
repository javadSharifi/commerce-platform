import React from 'react';
import { cn } from '../utils/cn.js';

export interface InputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'size'> {
  size?: 'sm' | 'md' | 'lg';
  isError?: boolean;
}

export const Input = React.forwardRef<HTMLInputElement, InputProps>(
  ({ className, size = 'md', isError = false, type = 'text', ...props }, ref) => {
    const sizeClass = {
      sm: 'input-sm',
      md: 'input-md',
      lg: 'input-lg',
    }[size];

    return (
      <input
        ref={ref}
        type={type}
        className={cn(
          'input input-bordered w-full text-base-content bg-base-100',
          sizeClass,
          isError && 'input-error',
          className
        )}
        {...props}
      />
    );
  }
);

Input.displayName = 'Input';
