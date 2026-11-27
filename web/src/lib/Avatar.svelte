<script lang="ts">
  let {
    name,
    src = '',
    size = 'sm',
    decorative = false
  }: {
    name: string;
    src?: string;
    size?: 'xs' | 'sm' | 'md';
    decorative?: boolean;
  } = $props();

  let failed = $state(false);

  let initials = $derived(
    name
      .trim()
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((part) => part[0]?.toUpperCase() ?? '')
      .join('') || '?'
  );
  let tone = $derived(hash(name) % 4);

  $effect(() => {
    src;
    failed = false;
  });

  function hash(value: string): number {
    let result = 0;
    for (let index = 0; index < value.length; index += 1) {
      result = (result * 31 + value.charCodeAt(index)) >>> 0;
    }
    return result;
  }
</script>

{#if src && !failed}
  <img
    class="avatar avatar-{size}"
    {src}
    alt={decorative ? '' : `${name}'s photo`}
    aria-hidden={decorative ? 'true' : undefined}
    onerror={() => (failed = true)}
  />
{:else}
  <span
    class="avatar avatar-{size} tone-{tone}"
    aria-hidden={decorative ? 'true' : undefined}
    role={decorative ? undefined : 'img'}
    aria-label={decorative ? undefined : name}
  >
    {initials}
  </span>
{/if}

<style>
  .avatar {
    flex: 0 0 auto;
    overflow: hidden;
    border: 1px solid var(--line);
    border-radius: var(--radius-chip);
    background: var(--media-bg);
    box-shadow: inset 0 1px 0 var(--highlight), var(--tc-shadow-1);
    object-fit: cover;
  }

  .avatar-xs {
    width: 2rem;
    height: 2rem;
  }

  .avatar-sm {
    width: 2.75rem;
    height: 2.75rem;
  }

  .avatar-md {
    width: 3.5rem;
    height: 3.5rem;
  }

  .avatar:not(img) {
    display: inline-grid;
    place-items: center;
    font-weight: 800;
    letter-spacing: -0.02em;
    line-height: 1;
  }

  .avatar-xs:not(img) {
    font-size: 0.78rem;
  }

  .avatar-sm:not(img) {
    font-size: 0.95rem;
  }

  .avatar-md:not(img) {
    font-size: 1.1rem;
  }

  .tone-0 {
    background: var(--court-deep);
    color: var(--ball);
  }

  .tone-1 {
    background: var(--ball-face);
    color: var(--ball-ink);
  }

  .tone-2 {
    background: var(--clay-deep);
    color: var(--on-danger);
  }

  .tone-3 {
    background: var(--info-surface);
    color: var(--info-ink);
  }
</style>
