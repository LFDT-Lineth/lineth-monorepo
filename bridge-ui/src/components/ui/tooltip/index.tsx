"use client";

import { useState } from "react";
import clsx from "clsx";
import styles from "./tooltip.module.scss";

type TooltipProps = {
  children: React.ReactNode;
  text: string;
  position?: "top" | "bottom" | "left" | "right";
};

export default function Tooltip({ children, text, position = "top" }: TooltipProps) {
  const [isVisible, setIsVisible] = useState(false);

  const showTooltip = () => setIsVisible(true);
  const hideTooltip = () => setIsVisible(false);

  return (
    <div className={styles["tooltip-wrapper"]}>
      <div className={styles["content-wrapper"]} onMouseEnter={showTooltip} onMouseLeave={hideTooltip}>
        {children}
      </div>
      <div
        className={clsx(styles["tooltip"], styles[position], {
          [styles["visible"]]: isVisible,
        })}
      >
        {text}
      </div>
    </div>
  );
}
