export function SlackIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="24"
      height="24"
      fill="currentColor"
      className={className}
      aria-hidden="true"
    >
      {/* Slack hash/pinwheel glyph: four rounded corners meeting at center */}
      <path d="M6.5 1a2.5 2.5 0 0 0-2.5 2.5v2.5A2.5 2.5 0 0 0 6.5 9h2.5V6.5A2.5 2.5 0 0 0 6.5 1Z" />
      <path d="M9 6.5A2.5 2.5 0 0 1 11.5 4h2.5a2.5 2.5 0 0 1 2.5 2.5v2.5a2.5 2.5 0 0 1-2.5 2.5H11.5V6.5Z" />
      <path d="M17.5 9a2.5 2.5 0 0 0 2.5-2.5v-2.5A2.5 2.5 0 0 0 17.5 1H15v2.5A2.5 2.5 0 0 0 17.5 9Z" />
      <path d="M15 11.5A2.5 2.5 0 0 1 17.5 14h2.5a2.5 2.5 0 0 1 2.5 2.5v2.5a2.5 2.5 0 0 1-2.5 2.5h-2.5v-2.5A2.5 2.5 0 0 1 15 11.5Z" />
      <path d="M6.5 15a2.5 2.5 0 0 0-2.5 2.5v2.5a2.5 2.5 0 0 0 2.5 2.5h2.5v-2.5A2.5 2.5 0 0 0 6.5 15Z" />
      <path d="M9 17.5A2.5 2.5 0 0 1 11.5 20h2.5a2.5 2.5 0 0 1 2.5 2.5v2.5a2.5 2.5 0 0 1-2.5 2.5h-2.5v-2.5A2.5 2.5 0 0 1 9 17.5Z" />
    </svg>
  );
}
