import {
  BookOpen,
  Bot,
  Brain,
  BriefcaseBusiness,
  ChartNoAxesColumnIncreasing,
  Code2,
  Coffee,
  Gamepad2,
  Globe2,
  GraduationCap,
  Heart,
  House,
  Lightbulb,
  MessagesSquare,
  Music2,
  Palette,
  Plane,
  Rocket,
  Sparkles,
  Star
} from "@lucide/svelte";

export const projectIconOptions = [
  { id: "messages", label: "Messages", component: MessagesSquare },
  { id: "sparkles", label: "Sparkles", component: Sparkles },
  { id: "lightbulb", label: "Idea", component: Lightbulb },
  { id: "book", label: "Book", component: BookOpen },
  { id: "briefcase", label: "Briefcase", component: BriefcaseBusiness },
  { id: "heart", label: "Heart", component: Heart },
  { id: "rocket", label: "Rocket", component: Rocket },
  { id: "palette", label: "Palette", component: Palette },
  { id: "globe", label: "Globe", component: Globe2 },
  { id: "chart", label: "Chart", component: ChartNoAxesColumnIncreasing },
  { id: "graduation", label: "Learning", component: GraduationCap },
  { id: "music", label: "Music", component: Music2 },
  { id: "gamepad", label: "Gaming", component: Gamepad2 },
  { id: "plane", label: "Travel", component: Plane },
  { id: "home", label: "Home", component: House },
  { id: "star", label: "Star", component: Star },
  { id: "coffee", label: "Coffee", component: Coffee },
  { id: "code", label: "Code", component: Code2 },
  { id: "bot", label: "Assistant", component: Bot },
  { id: "brain", label: "Thinking", component: Brain }
] as const;

export const projectColorOptions = [
  { value: "#ff7417", label: "Mistral orange" },
  { value: "#db6337", label: "Ember" },
  { value: "#e49b26", label: "Amber" },
  { value: "#d5b441", label: "Gold" },
  { value: "#4fa78f", label: "Mint" },
  { value: "#4b9db9", label: "Cyan" },
  { value: "#5c91d8", label: "Blue" },
  { value: "#8b78d1", label: "Violet" },
  { value: "#c56a91", label: "Rose" },
  { value: "#c56a55", label: "Clay" }
] as const;

export function projectIconComponent(icon?: string) {
  return projectIconOptions.find((option) => option.id === icon)?.component;
}

export function defaultProjectAppearance(kind: "chat" | "work") {
  return kind === "work"
    ? { icon: "briefcase", color: "#4fa78f" }
    : { icon: "messages", color: "#ff7417" };
}
