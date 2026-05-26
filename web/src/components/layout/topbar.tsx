import { useTranslation } from "react-i18next";
import { Link } from "@tanstack/react-router";
import { Search, User as UserIcon, LogOut } from "lucide-react";
import { ThemeToggle } from "@/components/layout/theme-toggle";
import { LangSwitch } from "@/components/layout/lang-switch";
import { Button } from "@/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import { useAuthStore } from "@/stores/auth-store";

/** Top navigation bar: logo, Cmd+K hint, theme toggle, lang switch, user menu. */
export function Topbar() {
  const { t } = useTranslation("common");
  const { user, clearSession } = useAuthStore();

  const handleLogout = () => {
    clearSession();
    window.location.href = "/";
  };

  return (
    <header
      className="flex h-14 items-center justify-between border-b border-[var(--color-border)] bg-[var(--glass-topbar)] px-4 backdrop-blur-xl"
      style={{ gridArea: "topbar" }}
    >
      {/* Left: logo */}
      <div className="flex items-center gap-3">
        <Link
          to="/dashboard"
          className="font-semibold text-[var(--color-text-primary)] text-[var(--font-size-lg)] transition-opacity hover:opacity-80"
        >
          {t("app_name")}
        </Link>
      </div>

      {/* Center: Cmd+K hint */}
      <button
        className="hidden md:flex items-center gap-2 rounded-xl border border-[var(--color-border)] bg-white/[.03] pl-3 pr-2 py-1.5 text-[13px] text-[var(--color-text-disabled)] hover:border-[var(--color-border-strong)] hover:bg-white/[.05] hover:text-[var(--color-text-tertiary)] transition-all duration-150 min-w-[220px]"
        onClick={() => document.dispatchEvent(new KeyboardEvent("keydown", { key: "k", metaKey: true }))}
      >
        <Search className="h-3.5 w-3.5 shrink-0" />
        <span className="flex-1 text-left">{t("actions.search")}</span>
        <kbd className="shrink-0 rounded-md border border-[var(--color-border)] bg-white/[.04] px-1.5 py-0.5 font-mono text-[10px]">⌘K</kbd>
      </button>

      {/* Right: controls */}
      <div className="flex items-center gap-1">
        <ThemeToggle />
        <LangSwitch />

        {user && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon" aria-label={t("nav.profile")}>
                <UserIcon className="h-4 w-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-48">
              <div className="px-2 py-1.5 text-xs text-[var(--color-text-tertiary)]">
                {user.username}
              </div>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={handleLogout}>
                <LogOut className="mr-2 h-4 w-4" />
                {t("actions.close")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </div>
    </header>
  );
}
