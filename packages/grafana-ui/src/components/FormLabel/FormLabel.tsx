import React, { FunctionComponent, ReactNode } from 'react';
import classNames from 'classnames';
import { Tooltip, PopoverContent } from '../Tooltip/Tooltip';
import { TooltipPlacement } from '../Tooltip/PopoverController';
import { Icon } from '../Icon/Icon';

interface Props {
  children: ReactNode;
  className?: string;
  htmlFor?: string;
  isFocused?: boolean;
  isInvalid?: boolean;
  tooltip?: PopoverContent;
  tooltipPlacement?: TooltipPlacement;
  width?: number | 'auto';
}

export const FormLabel: FunctionComponent<Props> = ({
  children,
  isFocused,
  isInvalid,
  className,
  htmlFor,
  tooltip,
  tooltipPlacement = 'top',
  width,
  ...rest
}) => {
  const classes = classNames(className, `gf-form-label width-${width ? width : '10'}`, {
    'gf-form-label--is-focused': isFocused,
    'gf-form-label--is-invalid': isInvalid,
  });

  return (
    <label className={classes} {...rest} htmlFor={htmlFor}>
      {children}
      {tooltip && (
        <Tooltip placement={tooltipPlacement} content={tooltip} theme={'info'}>
          <div className="gf-form-help-icon gf-form-help-icon--right-normal">
            <Icon name="info-circle" size="sm" style={{ marginLeft: '10px' }} />
          </div>
        </Tooltip>
      )}
    </label>
  );
};

export const InlineFormLabel = FormLabel;
