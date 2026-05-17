export interface AccentPreset {
  name: string;
  gradient: string;
  primary: string;
}

export const ACCENT_PRESETS: AccentPreset[] = [
  {
    name: "Iris",
    gradient: "linear-gradient(135deg, #6366f1 0%, #a855f7 50%, #ec4899 100%)",
    primary: "#6366f1",
  },
  {
    name: "Sunset",
    gradient: "linear-gradient(135deg, #f97316 0%, #ef4444 50%, #ec4899 100%)",
    primary: "#ef4444",
  },
  {
    name: "Ocean",
    gradient: "linear-gradient(135deg, #06b6d4 0%, #3b82f6 50%, #6366f1 100%)",
    primary: "#3b82f6",
  },
  {
    name: "Forest",
    gradient: "linear-gradient(135deg, #10b981 0%, #14b8a6 50%, #06b6d4 100%)",
    primary: "#10b981",
  },
  {
    name: "Mono",
    gradient: "linear-gradient(135deg, #404040 0%, #525252 50%, #0a0a0a 100%)",
    primary: "#404040",
  },
];

export interface FontPreset {
  name: string;
  value: string;
}

export const FONT_PRESETS: FontPreset[] = [
  {
    name: "现代",
    value: '"SF Pro Display", "PingFang SC", system-ui, sans-serif',
  },
  {
    name: "等宽",
    value: 'ui-monospace, "SF Mono", Menlo, monospace',
  },
  {
    name: "衬线",
    value: '"New York", "Songti SC", Georgia, serif',
  },
];

interface AccentPickerProps {
  value: string;
  onChange: (p: AccentPreset) => void;
}

export const AccentPicker = ({ value, onChange }: AccentPickerProps) => (
  <div className="accent-picker">
    {ACCENT_PRESETS.map((p) => (
      <button
        key={p.name}
        className={`accent-swatch ${p.gradient === value ? "active" : ""}`}
        style={{ background: p.gradient }}
        title={p.name}
        onClick={() => onChange(p)}
      />
    ))}
  </div>
);

interface FontPickerProps {
  value: string;
  onChange: (p: FontPreset) => void;
}

export const FontPicker = ({ value, onChange }: FontPickerProps) => (
  <div className="font-picker">
    {FONT_PRESETS.map((f) => (
      <button
        key={f.name}
        className={`font-chip ${f.value === value ? "active" : ""}`}
        style={{ fontFamily: f.value }}
        onClick={() => onChange(f)}
      >
        Aa · {f.name}
      </button>
    ))}
  </div>
);
