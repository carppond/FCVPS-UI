import { type ClassValue, clsx } from "clsx";
import { twMerge } from "tailwind-merge";

/** Merges Tailwind class names, resolving conflicts via tailwind-merge. */
export const cn = (...inputs: ClassValue[]) => twMerge(clsx(inputs));
