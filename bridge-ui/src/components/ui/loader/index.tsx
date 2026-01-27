import { ComponentPropsWithoutRef } from "react";

import clsx from "clsx";

import styles from "./loader.module.scss";

type LoaderProps = ComponentPropsWithoutRef<"div"> & {
  fill?: string;
};

export function Loader({ fill = "#000", className }: LoaderProps) {
  return (
    <div className={clsx(styles.loader, className)}>
      <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
        <path
          d="M12,20.75c-.23,0-.46,0-.69-.03-1.61-.13-3.15-.7-4.46-1.65-1.31-.95-2.32-2.24-2.94-3.73-.62-1.49-.81-3.12-.56-4.72.25-1.59.94-3.09,1.99-4.31,1.05-1.23,2.41-2.14,3.95-2.64.39-.13.82.09.95.48.13.39-.09.82-.48.95-1.27.41-2.4,1.17-3.27,2.19-.87,1.02-1.44,2.25-1.65,3.57-.21,1.32-.05,2.67.46,3.91.51,1.24,1.35,2.3,2.44,3.09,1.08.79,2.36,1.26,3.69,1.36,1.33.11,2.67-.16,3.86-.77,1.19-.61,2.19-1.53,2.89-2.67s1.07-2.45,1.07-3.79c0-.41.34-.75.75-.75s.75.34.75.75c0,1.61-.45,3.19-1.29,4.57s-2.05,2.49-3.49,3.22c-1.23.63-2.59.95-3.97.95Z"
          fill={fill}
        />
      </svg>
    </div>
  );
}
