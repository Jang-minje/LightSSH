import { useState } from "react";
import {
  defaultAppearanceSettings,
  fontChoices,
  type AppearanceSettings,
} from "../api/settings";

type SettingsModalProps = {
  settings: AppearanceSettings;
  onChange: (settings: AppearanceSettings) => void;
  onClose: () => void;
};

export function SettingsModal({
  settings,
  onChange,
  onClose,
}: SettingsModalProps) {
  const [tab, setTab] = useState<"style" | "font">("style");

  const update = (changes: Partial<AppearanceSettings>) => {
    onChange({ ...settings, ...changes });
  };

  return (
    <div className="modal-backdrop" role="presentation">
      <section className="modal settings-modal" role="dialog" aria-modal="true">
        <div className="modal-title-row">
          <h2>옵션</h2>
          <button type="button" className="secondary-button" onClick={onClose}>
            닫기
          </button>
        </div>
        <div className="settings-tabs">
          <button
            type="button"
            className={tab === "style" ? "settings-tab active" : "settings-tab"}
            onClick={() => setTab("style")}
          >
            스타일
          </button>
          <button
            type="button"
            className={tab === "font" ? "settings-tab active" : "settings-tab"}
            onClick={() => setTab("font")}
          >
            글꼴
          </button>
        </div>

        {tab === "style" ? (
          <div className="settings-section">
            <div className="segmented-control" aria-label="테마">
              <button
                type="button"
                className={settings.theme === "light" ? "active" : ""}
                onClick={() => update({ theme: "light" })}
              >
                라이트
              </button>
              <button
                type="button"
                className={settings.theme === "dark" ? "active" : ""}
                onClick={() => update({ theme: "dark" })}
              >
                다크
              </button>
            </div>
          </div>
        ) : (
          <div className="settings-section font-grid">
            <fieldset>
              <legend>프로그램</legend>
              <label>
                글꼴
                <select
                  value={settings.appFontFamily}
                  onChange={(event) =>
                    update({ appFontFamily: event.target.value })
                  }
                >
                  {fontChoices.map((font) => (
                    <option key={font.family} value={font.family}>
                      {font.label}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                크기
                <input
                  type="number"
                  min="12"
                  max="20"
                  value={settings.appFontSize}
                  onChange={(event) =>
                    update({ appFontSize: Number(event.target.value) })
                  }
                />
              </label>
            </fieldset>
            <fieldset>
              <legend>터미널</legend>
              <label>
                글꼴
                <select
                  value={settings.terminalFontFamily}
                  onChange={(event) =>
                    update({ terminalFontFamily: event.target.value })
                  }
                >
                  {fontChoices.map((font) => (
                    <option key={font.family} value={font.family}>
                      {font.label}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                크기
                <input
                  type="number"
                  min="10"
                  max="28"
                  value={settings.terminalFontSize}
                  onChange={(event) =>
                    update({ terminalFontSize: Number(event.target.value) })
                  }
                />
              </label>
            </fieldset>
          </div>
        )}
        <button
          type="button"
          className="secondary-button"
          onClick={() => onChange(defaultAppearanceSettings)}
        >
          기본값
        </button>
      </section>
    </div>
  );
}
