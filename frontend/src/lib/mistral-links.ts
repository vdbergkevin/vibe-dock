export const mistralLinks = {
  connectors: "https://admin.mistral.ai/organization/connectors",
  apiKeys: "https://console.mistral.ai/home?profile_dialog=api-keys",
  usage: "https://admin.mistral.ai/organization/usage",
  subscription: "https://admin.mistral.ai/subscription",
  billing: "https://admin.mistral.ai/organization/billing",
  connectorDebugger: "https://console.mistral.ai/build/connectors/debugger",
  libraries: "https://chat.mistral.ai/libraries",
  work: "https://chat.mistral.ai/work"
} as const;

export type MistralDestination = keyof typeof mistralLinks;
