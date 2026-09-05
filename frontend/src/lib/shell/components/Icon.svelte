<script lang="ts" module>
  // Hand-rolled 24px stroke icon set — no icon package, no external requests.
  const PATHS = {
    play: 'M8 5.2v13.6L19 12z',
    plus: 'M12 5v14M5 12h14',
    import: 'M12 3v10m0 0 4-4m-4 4-4-4M4 16v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3',
    copy: 'M8 8h11a1 1 0 0 1 1 1v11a1 1 0 0 1-1 1H8a1 1 0 0 1-1-1V9a1 1 0 0 1 1-1zM4 16V4a1 1 0 0 1 1-1h11',
    pencil: 'M4 20h4L18.5 9.5a2.5 2.5 0 0 0-3.5-3.5L4.5 16.5 4 20z',
    trash: 'M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M6.5 7l.9 13a1 1 0 0 0 1 1h7.2a1 1 0 0 0 1-1l.9-13',
    link: 'M10.5 13.5a4 4 0 0 0 5.7 0l2.5-2.5a4 4 0 0 0-5.7-5.7l-1.4 1.4M13.5 10.5a4 4 0 0 0-5.7 0l-2.5 2.5a4 4 0 0 0 5.7 5.7l1.4-1.4',
    gear: 'M12 15.2a3.2 3.2 0 1 0 0-6.4 3.2 3.2 0 0 0 0 6.4zM19.4 15a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.6 1.6 0 0 0-2.7 1.1v.3a2 2 0 1 1-4 0v-.2a1.6 1.6 0 0 0-2.8-1.1l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1A1.6 1.6 0 0 0 3.5 14h-.3a2 2 0 1 1 0-4h.2a1.6 1.6 0 0 0 1.1-2.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.6 1.6 0 0 0 2.7-1.1v-.3a2 2 0 1 1 4 0v.2a1.6 1.6 0 0 0 2.8 1.1l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0 1.1 2.7h.3a2 2 0 1 1 0 4h-.2a1.6 1.6 0 0 0-1.4 1z',
    refresh: 'M20 12a8 8 0 1 1-2.4-5.7M20 4v5h-5',
    chevronRight: 'm9.5 6 6 6-6 6',
    chevronLeft: 'm14.5 6-6 6 6 6',
    chevronUp: 'm6 14.5 6-6 6 6',
    chevronDown: 'm6 9.5 6 6 6-6',
    search: 'M17 11a6 6 0 1 1-12 0 6 6 0 0 1 12 0zM20.5 20.5 15.5 15.5',
    x: 'M6.5 6.5 17.5 17.5M17.5 6.5 6.5 17.5',
    check: 'm5.5 12.5 4.5 4.5L18.5 7',
    alert: 'M12 4 2.7 20h18.6L12 4zM12 10.5v4.2M12 17.6h.01',
    info: 'M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0zM12 11v5.5M12 7.8h.01',
    grip: 'M9.5 6h.01M9.5 12h.01M9.5 18h.01M14.5 6h.01M14.5 12h.01M14.5 18h.01',
    folder: 'M3 7a2 2 0 0 1 2-2h3.6l2 2H19a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7z',
    save: 'M5 4h10.5L19 7.5V20H5V4zM8.5 4v5.5h7V4M8.5 20v-6h7v6',
    sort: 'M7 4.5v15m0 0-3-3m3 3 3-3M17 19.5v-15m0 0-3 3m3-3 3 3',
    undo: 'M4.5 9.5h10a5 5 0 0 1 0 10h-3.5M4.5 9.5l4-4M4.5 9.5l4 4',
    moveAll: 'M4 12h10m0 0-3.5-3.5M14 12l-3.5 3.5M19 5v14',
    kebab: 'M12 5.5h.01M12 12h.01M12 18.5h.01',
    panelLeft:
      'M4 6a2 2 0 0 1 2-2h12a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6zM9.5 4.2v15.6',
    menu: 'M4 7h16M4 12h16M4 17h16',
    spinner: 'M12 3a9 9 0 0 1 9 9',
    // Game marks for the rail.
    // A ringed planet: the ring is one arc that ducks behind the planet, so
    // the planet sits in front of its own rim.
    rimworld: 'M12 18.1a5.7 5.7 0 1 0 0-11.4 5.7 5.7 0 0 0 0 11.4zM6.3 11.7A11.3 3.6 -28 1 0 14.6 7.3',
    // A grass block: an isometric cube with the grass layer hanging down the
    // side faces in uneven pixel steps.
    minecraft:
      'M12 3 3.5 7.5v9L12 21l8.5-4.5v-9L12 3zM3.5 7.5 12 12l8.5-4.5M12 12v9M3.5 10.2l3.8 2v2l4.7 2.5 3.4-1.8v-2l5.1-2.7',
    download: 'M12 4v11m0 0 4-4m-4 4-4-4M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2',
    externalLink: 'M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5'
  } as const;

  export type IconName = keyof typeof PATHS;
  export const iconNames = Object.keys(PATHS) as IconName[];
  export { PATHS };
</script>

<script lang="ts">
  interface Props {
    name: IconName;
    size?: number;
    strokeWidth?: number;
    class?: string;
  }
  let { name, size = 16, strokeWidth = 1.8, class: klass = '' }: Props = $props();

  const filled = $derived(name === 'play');
  const dotty = $derived(name === 'grip' || name === 'kebab');
</script>

<svg
  class={klass}
  width={size}
  height={size}
  viewBox="0 0 24 24"
  fill={filled ? 'currentColor' : 'none'}
  stroke="currentColor"
  stroke-width={dotty ? 2.4 : strokeWidth}
  stroke-linecap="round"
  stroke-linejoin="round"
  aria-hidden="true"
  focusable="false"
>
  <path d={PATHS[name]} />
</svg>

<style>
  svg {
    flex: none;
    display: block;
  }
</style>
