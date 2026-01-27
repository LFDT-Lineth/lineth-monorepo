"use client";
import { default as NextImage, ImageProps, StaticImageData } from "next/image";

import styles from "./image.module.scss";

type Props = Omit<ImageProps, "src"> & {
  src: string | StaticImageData;
};

type LoaderProps = Pick<Props, "src" | "width" | "quality">;

const contentfulLoader = ({ src, width, quality }: LoaderProps) => `${src}?w=${width}&q=${quality || 75}&fm=avif`;

const isContentfulImage = (src: string | StaticImageData) => {
  if (typeof src !== "string") return false;

  const normalizedUrl = src.startsWith("//") ? `https:${src}` : src;

  try {
    const url = new URL(normalizedUrl);
    return url.hostname === "images.ctfassets.net";
  } catch {
    return false;
  }
};

const Image = ({
  src,
  width,
  height,
  alt,
  fill = false,
  priority = false,
  sizes,
  loading = "lazy",
  quality = 75,
  unoptimized,
  ...props
}: Props) => {
  if (!src) return null;

  return (
    <NextImage
      className={styles["image"]}
      src={src}
      alt={alt}
      width={!fill ? width : undefined}
      height={!fill ? height : undefined}
      sizes={sizes}
      fill={fill}
      loading={loading}
      unoptimized={unoptimized || (typeof src === "string" && src.includes(".svg"))}
      loader={isContentfulImage(src) ? ({ src, width }) => contentfulLoader({ src, width, quality }) : undefined}
      priority={priority}
      {...props}
    />
  );
};

export default Image;
