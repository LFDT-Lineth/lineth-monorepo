import { clsx } from "clsx";
import React, { forwardRef, ButtonHTMLAttributes, ReactNode } from "react";

import styles from "./button.module.scss";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "outline" | "link";
  children: ReactNode;
  disabled?: boolean;
  fullWidth?: boolean;
}

const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ variant = "primary", onClick, children, disabled, className, fullWidth, type = "button", ...rest }, ref) => {
    const buttonClassNames = clsx(styles["button"], styles[variant], className, {
      [styles["full-width"]]: fullWidth,
      [styles["disabled"]]: disabled,
    });

    return (
      <button ref={ref} onClick={onClick} type={type} className={buttonClassNames} disabled={disabled} {...rest}>
        {children}
      </button>
    );
  },
);

Button.displayName = "Button";

export default Button;
