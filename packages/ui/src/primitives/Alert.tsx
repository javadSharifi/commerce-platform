import React from 'react';
import { cn } from '../utils/cn.js';

export interface AlertProps extends React.HTMLAttributes<HTMLDivElement> {
  variant?: 'info' | 'success' | 'warning' | 'error';
  icon?: React.ReactNode;
  title?: string;
}

export const Alert: React.FC<AlertProps> = ({
  children,
  className,
  variant = 'info',
  icon,
  title,
  ...props
}) => {
  const variantClass = {
    info: 'alert-info',
    success: 'alert-success',
    warning: 'alert-warning',
    error: 'alert-error',
  }[variant];

  return (
    <div role="alert" className={cn('alert', variantClass, className)} {...props}>
      {icon && <span className="shrink-0">{icon}</span>}
      <div className="flex-1">
        {title && <h4 className="font-bold mb-0.5">{title}</h4>}
        {children}
      </div>
    </div>
  );
};
