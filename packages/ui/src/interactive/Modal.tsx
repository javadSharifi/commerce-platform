import React from 'react';
import {
  DialogTrigger,
  ModalOverlay as RACModalOverlay,
  Modal as RACModal,
  Dialog as RACDialog,
  Heading as RACHeading,
  type ModalOverlayProps as RACModalOverlayProps,
  type DialogProps as RACDialogProps,
} from 'react-aria-components';
import { cn } from '../utils/cn.js';

export interface ModalProps {
  isOpen?: boolean;
  defaultOpen?: boolean;
  onOpenChange?: (isOpen: boolean) => void;
  trigger?: React.ReactNode;
  title?: React.ReactNode;
  description?: React.ReactNode;
  children?: React.ReactNode;
  actions?: React.ReactNode;
  size?: 'sm' | 'md' | 'lg' | 'xl';
  isDismissable?: boolean;
  className?: string;
  ariaLabel?: string;
}

const sizeClasses: Record<'sm' | 'md' | 'lg' | 'xl', string> = {
  sm: 'max-w-sm',
  md: 'max-w-md',
  lg: 'max-w-lg',
  xl: 'max-w-2xl',
};

/**
 * Accessible Modal dialog built on React Aria Components and styled with DaisyUI tokens.
 * Fully compatible with SSR and React 19.
 */
export const Modal: React.FC<ModalProps> = ({
  isOpen,
  defaultOpen,
  onOpenChange,
  trigger,
  title,
  description,
  children,
  actions,
  size = 'md',
  isDismissable = true,
  className,
  ariaLabel,
}) => {
  const content = (
    <RACModalOverlay
      isOpen={isOpen}
      defaultOpen={defaultOpen}
      onOpenChange={onOpenChange}
      isDismissable={isDismissable}
      className={({ isEntering, isExiting }) =>
        cn(
          'fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-xs p-4 overflow-y-auto duration-200',
          isEntering && 'animate-in fade-in',
          isExiting && 'animate-out fade-out'
        )
      }
    >
      <RACModal
        className={({ isEntering, isExiting }) =>
          cn(
            'w-full bg-base-100 text-base-content rounded-box shadow-2xl border border-base-200 p-6 outline-none duration-200 my-auto',
            sizeClasses[size],
            isEntering && 'zoom-in-95',
            isExiting && 'zoom-out-95',
            className
          )
        }
      >
        <RACDialog
          aria-label={typeof title === 'string' ? title : ariaLabel || 'دیالوگ'}
          className="outline-none text-start"
        >
          {({ close }) => (
            <>
              {title && (
                <RACHeading
                  slot="title"
                  className="text-lg font-bold text-base-content mb-2"
                >
                  {title}
                </RACHeading>
              )}
              {description && (
                <p className="text-sm text-base-content/70 mb-4">{description}</p>
              )}
              <div className="text-base-content text-sm">{children}</div>
              {actions && (
                <div className="modal-action mt-6 flex justify-end gap-2">{actions}</div>
              )}
            </>
          )}
        </RACDialog>
      </RACModal>
    </RACModalOverlay>
  );

  if (trigger) {
    return (
      <DialogTrigger
        isOpen={isOpen}
        defaultOpen={defaultOpen}
        onOpenChange={onOpenChange}
      >
        {trigger}
        {content}
      </DialogTrigger>
    );
  }

  return content;
};

// Re-exports for advanced headless composition
export {
  DialogTrigger,
  RACModalOverlay as ModalOverlay,
  RACModal as ModalContent,
  RACDialog as Dialog,
  RACHeading as ModalHeading,
};
