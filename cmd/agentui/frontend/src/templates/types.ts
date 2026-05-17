import type { ComponentType, CSSProperties } from "react";
import type { UIComponentPayload } from "../types/bindings";

export interface TemplatePreviewProps {
  accent: string;
  accentPrimary: string;
  fontFamily: string;
  style?: CSSProperties;
}

export interface TemplateSpec {
  id: string;
  name: string;
  description: string;
  Preview: ComponentType<TemplatePreviewProps>;
  components: UIComponentPayload[];
}

export const cssVars = (accent: string, accentPrimary: string, font: string) =>
  ({
    ["--canvas-accent" as any]: accent,
    ["--canvas-accent-primary" as any]: accentPrimary,
    ["--canvas-font" as any]: font,
  } as CSSProperties);
