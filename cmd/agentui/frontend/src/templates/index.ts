import "./styles.css";
import { dashboardTemplate } from "./dashboard";
import { chatTemplate } from "./chat";
import { authTemplate } from "./auth";
import { landingTemplate } from "./landing";
import { tableTemplate } from "./table";
import type { TemplateSpec } from "./types";

export const TEMPLATES: TemplateSpec[] = [
  dashboardTemplate,
  chatTemplate,
  authTemplate,
  landingTemplate,
  tableTemplate,
];

export const getTemplate = (id: string): TemplateSpec | undefined =>
  TEMPLATES.find((t) => t.id === id);

export type { TemplateSpec } from "./types";
