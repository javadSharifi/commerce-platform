import React from 'react';
import { cn } from '../utils/cn.js';

export interface TextareaProps extends React.TextareaHTMLAttributes<HTMLTextAreaElement> {
  size?: 'sm' | 'md' | 'lg';
  isError?: boolean;
}

export const Textarea = React.forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ className, size = 'md', isError = false, ...props }, ref) => {
    const sizeClass = {
      sm: 'textarea-sm',
      md: 'textarea-md',
      lg: 'textarea-lg',
    }[size];

    return (
      <textarea
        ref={ref}
        className={cn(
          'textarea textarea-bordered w-full text-base-content bg-base-100',
          sizeClass,
          isError && 'textarea-error',
          className
        )}
        {...props}
      />
    );
  }
);

Textarea.displayName = 'Textarea';
