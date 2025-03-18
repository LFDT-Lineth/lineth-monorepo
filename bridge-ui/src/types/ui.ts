export type AssetType = {
  description?: string;
  file: {
    url: string;
    details: {
      size?: number;
      image: {
        width: number;
        height: number;
      };
    };
    contentType?: string;
    fileName?: string;
  };
  title: string;
};

export type LinkBlock = {
  name: string;
  label: string;
  url?: string;
  external?: boolean;
  active?: boolean;
  icon?: AssetType;
  submenusLeft?: LinkBlock[];
  submenusRight?: LinkBlock;
  mobileOnly?: boolean;
  desktopUrl?: string;
};

export enum Theme {
  "default" = "default",
  "navy" = "navy",
  "cyan" = "cyan",
  "indigo" = "indigo",
  "tangerine" = "tangerine",
}
