import React from 'react';
import { cn } from '../utils/cn.js';

export interface EmptyStateProps extends React.HTMLAttributes<HTMLDivElement> {
  icon?: React.ReactNode;
  title: string;
  description?: string;
  action?: React.ReactNode;
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  className,
  icon,
  title,
  description,
  action,
  children,
  ...props
}) => {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center p-8 text-center bg-base-100 rounded-box border border-base-200 text-base-content',
        className
      )}
      {...props}
    >
      {icon && <div className="text-4xl text-base-content/50 mb-3">{icon}</div>}
      <h3 className="text-lg font-bold text-base-content mb-1">{title}</h3>
      {description && <p className="text-sm text-base-content/70 max-w-md mb-4">{description}</p>}
      {action && <div className="mt-2">{action}</div>}
      {children && <div className="mt-2">{children}</div>}
    </div>
  );
};
