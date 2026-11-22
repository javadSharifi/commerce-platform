import React from 'react';
import {
  Tabs as RACTabs,
  TabList as RACTabList,
  Tab as RACTab,
  TabPanel as RACTabPanel,
  type TabsProps as RACTabsProps,
  type TabListProps as RACTabListProps,
  type TabProps as RACTabProps,
  type TabPanelProps as RACTabPanelProps,
} from 'react-aria-components';
import { cn } from '../utils/cn.js';

export interface TabItem {
  id: string;
  label: React.ReactNode;
  content: React.ReactNode;
  disabled?: boolean;
}

export interface TabsProps extends Omit<RACTabsProps, 'children'> {
  variant?: 'bordered' | 'boxed' | 'lifted';
  size?: 'sm' | 'md' | 'lg';
  items?: TabItem[];
  children?: React.ReactNode;
}

const variantClasses: Record<'bordered' | 'boxed' | 'lifted', string> = {
  bordered: 'tabs-bordered',
  boxed: 'tabs-boxed bg-base-200 p-1 rounded-box',
  lifted: 'tabs-lifted',
};

const sizeClasses: Record<'sm' | 'md' | 'lg', string> = {
  sm: 'tabs-sm',
  md: 'tabs-md',
  lg: 'tabs-lg',
};

/**
 * Accessible Tabs component built on React Aria Components and styled with DaisyUI tabs.
 */
export const Tabs: React.FC<TabsProps> = ({
  variant = 'boxed',
  size = 'md',
  items,
  children,
  className,
  ...props
}) => {
  return (
    <RACTabs className={cn('w-full', className)} {...props}>
      {items ? (
        <>
          <TabList variant={variant} size={size}>
            {items.map((item) => (
              <Tab key={item.id} id={item.id} isDisabled={item.disabled}>
                {item.label}
              </Tab>
            ))}
          </TabList>
          {items.map((item) => (
            <TabPanel key={item.id} id={item.id}>
              {item.content}
            </TabPanel>
          ))}
        </>
      ) : (
        children
      )}
    </RACTabs>
  );
};

export interface TabListProps<T extends object = object> extends RACTabListProps<T> {
  variant?: 'bordered' | 'boxed' | 'lifted';
  size?: 'sm' | 'md' | 'lg';
}

export const TabList = <T extends object = object>({
  variant = 'boxed',
  size = 'md',
  className,
  children,
  ...props
}: TabListProps<T>) => {
  return (
    <RACTabList
      className={cn(
        'tabs',
        variantClasses[variant],
        sizeClasses[size],
        className
      )}
      {...props}
    >
      {children}
    </RACTabList>
  );
};

export interface TabProps extends RACTabProps {}

export const Tab: React.FC<TabProps> = ({ className, children, ...props }) => {
  return (
    <RACTab
      className={(renderProps) =>
        cn(
          'tab transition-all duration-150 focus:outline-none cursor-pointer text-sm',
          renderProps.isSelected && 'tab-active font-bold text-base-content',
          renderProps.isDisabled && 'tab-disabled opacity-40 cursor-not-allowed',
          renderProps.isFocused && 'ring-2 ring-primary/30',
          typeof className === 'function'
            ? className(renderProps)
            : className
        )
      }
      {...props}
    >
      {children}
    </RACTab>
  );
};

export interface TabPanelProps extends RACTabPanelProps {}

export const TabPanel: React.FC<TabPanelProps> = ({
  className,
  children,
  ...props
}) => {
  return (
    <RACTabPanel
      className={cn('outline-none mt-4 text-base-content text-start', className)}
      {...props}
    >
      {children}
    </RACTabPanel>
  );
};

// Re-exports
export {
  RACTabs as TabsRoot,
  RACTabList as BaseTabList,
  RACTab as BaseTab,
  RACTabPanel as BaseTabPanel,
};
