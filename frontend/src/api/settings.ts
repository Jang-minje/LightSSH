import { app } from "./wails";

export type AppTheme = "light" | "dark";

export type FontChoice = {
  label: string;
  family: string;
};

export type AppearanceSettings = {
  theme: AppTheme;
  appFontFamily: string;
  appFontSize: number;
  terminalFontFamily: string;
  terminalFontSize: number;
};

export const fontChoices: FontChoice[] = [
  { label: "맑은 고딕", family: '"Malgun Gothic", "Segoe UI", sans-serif' },
  { label: "굴림", family: "Gulim, sans-serif" },
  { label: "돋움", family: "Dotum, sans-serif" },
  { label: "나눔고딕", family: '"Nanum Gothic", "Malgun Gothic", sans-serif' },
  { label: "D2Coding", family: "D2Coding, Consolas, monospace" },
  { label: "Cascadia Mono", family: '"Cascadia Mono", Consolas, monospace' },
  { label: "Consolas", family: "Consolas, monospace" },
];

export const defaultAppearanceSettings: AppearanceSettings = {
  theme: "light",
  appFontFamily: fontChoices[0].family,
  appFontSize: 14,
  terminalFontFamily: fontChoices[5].family,
  terminalFontSize: 14,
};

export async function loadAppearanceSettings(): Promise<AppearanceSettings> {
  return normalizeSettings(await app().LoadAppearanceSettings());
}

export async function saveAppearanceSettings(
  settings: AppearanceSettings,
): Promise<void> {
  await app().SaveAppearanceSettings(normalizeSettings(settings));
}

function normalizeSettings(
  settings: Partial<AppearanceSettings>,
): AppearanceSettings {
  return {
    theme: settings.theme === "dark" ? "dark" : "light",
    appFontFamily:
      settings.appFontFamily || defaultAppearanceSettings.appFontFamily,
    appFontSize: clamp(settings.appFontSize, 12, 20, 14),
    terminalFontFamily:
      settings.terminalFontFamily ||
      defaultAppearanceSettings.terminalFontFamily,
    terminalFontSize: clamp(settings.terminalFontSize, 10, 28, 14),
  };
}

function clamp(
  value: number | undefined,
  min: number,
  max: number,
  fallback: number,
): number {
  if (!Number.isFinite(value)) {
    return fallback;
  }
  return Math.min(max, Math.max(min, Number(value)));
}
