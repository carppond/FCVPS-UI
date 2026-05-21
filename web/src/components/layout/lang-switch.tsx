import { useTranslation } from "react-i18next";
import { Globe } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { Button } from "@/components/ui/button";

/**
 * Native language labels — i18n-lint: native-name.
 *
 * Each entry shows the language in its own script as a UX convention
 * (mirrors Google/GitHub language switchers). These CJK / Hangul literals
 * are intentional and exempt from the no-hardcoded-CJK rule; the i18n
 * check-script whitelists this file by path.
 */
const LANGUAGES: { code: string; label: string }[] = [
  { code: "zh-CN", label: "中文（简体）" },
  { code: "en", label: "English" },
  { code: "ja", label: "日本語" },
  { code: "ko", label: "한국어" },
];

/** Four-language switcher that persists the selection to localStorage. */
export function LangSwitch() {
  const { t, i18n } = useTranslation(["common"]);
  const currentLang = i18n.language;

  const handleChange = (lang: string) => {
    void i18n.changeLanguage(lang);
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="ghost" size="icon" aria-label={t("common:aria.lang")}>
          <Globe className="h-4 w-4" />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuRadioGroup value={currentLang} onValueChange={handleChange}>
          {LANGUAGES.map((lang) => (
            <DropdownMenuRadioItem key={lang.code} value={lang.code}>
              {lang.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
