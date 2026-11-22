import React from 'react';
import { cn } from '../utils/cn.js';

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'accent' | 'ghost' | 'outline';
  size?: 'sm' | 'md' | 'lg';
  fullWidth?: boolean;
  isLoading?: boolean;
}

export const Button: React.FC<ButtonProps> = ({
  children,
  className,
  variant = 'primary',
  size = 'md',
  fullWidth = false,
  isLoading = false,
  disabled,
  type = 'button',
  ...props
}) => {
  const variantClass = {
    primary: 'btn-primary',
    secondary: 'btn-secondary',
    accent: 'btn-accent',
    ghost: 'btn-ghost',
    outline: 'btn-outline',
  }[variant];

  const sizeClass = {
    sm: 'btn-sm',
    md: 'btn-md',
    lg: 'btn-lg',
  }[size];

  return (
    <button
      type={type}
      disabled={disabled || isLoading}
      className={cn('btn', variantClass, sizeClass, fullWidth && 'w-full', className)}
      {...props}
    >
      {isLoading && <span className="loading loading-spinner loading-xs me-2" aria-hidden="true" />}
      {children}
    </button>
  );
};
