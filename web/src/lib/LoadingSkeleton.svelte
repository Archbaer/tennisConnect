<script lang="ts">
  export let variant: 'dashboard' | 'discover' | 'matches' | 'match' | 'player' | 'profile' | 'settings';
  export let label = 'Loading…';
</script>

<section class="skeleton-region stack" aria-busy="true">
  <p class="visually-hidden" role="status">{label}</p>
  <div class="skeleton-layout" aria-hidden="true">
    {#if variant === 'dashboard'}
      <div class="sk-kpis">
        {#each [0, 1, 2, 3] as item}
          <div class="sk-panel sk-kpi" data-item={item}>
            <span class="skeleton sk-line short"></span>
            <span class="skeleton sk-line value"></span>
            <span class="skeleton sk-line pill"></span>
          </div>
        {/each}
      </div>
      <div class="sk-split">
        <div class="sk-panel">
          <span class="skeleton sk-line short"></span>
          <span class="skeleton sk-line title"></span>
          <span class="skeleton sk-chart"></span>
        </div>
        <div class="sk-panel">
          <span class="skeleton sk-line short"></span>
          <span class="skeleton sk-line title"></span>
          <span class="skeleton sk-line value"></span>
          <span class="skeleton sk-line"></span>
          <span class="skeleton sk-line medium"></span>
          <span class="skeleton sk-line short"></span>
        </div>
      </div>
      <div class="sk-panel">
        <span class="skeleton sk-line title"></span>
        {#each [0, 1, 2, 3] as row}
          <div class="sk-row" data-row={row}>
            <span class="skeleton sk-dot"></span>
            <span class="skeleton sk-line"></span>
            <span class="skeleton sk-line short"></span>
          </div>
        {/each}
      </div>
    {:else if variant === 'matches'}
      <div class="sk-kpis three">
        {#each [0, 1, 2] as item}
          <div class="sk-panel sk-kpi" data-item={item}>
            <span class="skeleton sk-line short"></span>
            <span class="skeleton sk-line value"></span>
            <span class="skeleton sk-line pill"></span>
          </div>
        {/each}
      </div>
      <div class="sk-panel">
        <span class="skeleton sk-line title"></span>
        {#each [0, 1, 2, 3] as row}
          <div class="sk-row" data-row={row}>
            <span class="skeleton sk-dot"></span>
            <span class="skeleton sk-line"></span>
            <span class="skeleton sk-line short"></span>
          </div>
        {/each}
      </div>
    {:else if variant === 'discover'}
      <div class="sk-panel sk-media">
        <span class="skeleton sk-photo"></span>
        <div class="sk-body">
          <span class="skeleton sk-line title"></span>
          <span class="skeleton sk-line medium"></span>
          <span class="skeleton sk-line"></span>
          <div class="sk-actions">
            <span class="skeleton sk-button"></span>
            <span class="skeleton sk-button"></span>
          </div>
        </div>
      </div>
    {:else if variant === 'player'}
      <div class="sk-panel sk-media">
        <span class="skeleton sk-photo"></span>
        <div class="sk-body">
          <span class="skeleton sk-line title"></span>
          <span class="skeleton sk-line"></span>
          <span class="skeleton sk-line medium"></span>
        </div>
      </div>
      <div class="sk-kpis three">
        {#each [0, 1, 2] as item}
          <div class="sk-panel sk-kpi" data-item={item}>
            <span class="skeleton sk-line short"></span>
            <span class="skeleton sk-line value"></span>
          </div>
        {/each}
      </div>
      <div class="sk-panel">
        <span class="skeleton sk-line title"></span>
        <span class="skeleton sk-line"></span>
        <span class="skeleton sk-line medium"></span>
      </div>
    {:else if variant === 'match'}
      <div class="sk-kpis">
        {#each [0, 1, 2, 3] as item}
          <div class="sk-panel sk-kpi" data-item={item}>
            <span class="skeleton sk-line short"></span>
            <span class="skeleton sk-line value"></span>
          </div>
        {/each}
      </div>
      <div class="sk-panel">
        <span class="skeleton sk-line title"></span>
        <span class="skeleton sk-line"></span>
        <span class="skeleton sk-line medium"></span>
        <div class="sk-actions">
          <span class="skeleton sk-button"></span>
          <span class="skeleton sk-button"></span>
        </div>
      </div>
    {:else if variant === 'profile'}
      <div class="sk-kpis">
        {#each [0, 1, 2, 3] as item}
          <div class="sk-panel sk-kpi" data-item={item}>
            <span class="skeleton sk-line short"></span>
            <span class="skeleton sk-line value"></span>
            <span class="skeleton sk-line pill"></span>
          </div>
        {/each}
      </div>
      <div class="sk-split">
        <div class="sk-panel sk-media">
          <span class="skeleton sk-photo"></span>
          <div class="sk-body">
            <span class="skeleton sk-line title"></span>
            <span class="skeleton sk-line"></span>
            <span class="skeleton sk-line medium"></span>
          </div>
        </div>
        <div class="sk-panel">
          <span class="skeleton sk-line title"></span>
          <span class="skeleton sk-line"></span>
          <span class="skeleton sk-line medium"></span>
          <span class="skeleton sk-line short"></span>
        </div>
      </div>
    {:else}
      <div class="sk-settings">
        {#each [0, 1, 2, 3, 4, 5] as card}
          <div class="sk-panel" data-card={card}>
            <span class="skeleton sk-line title"></span>
            <span class="skeleton sk-line"></span>
            <span class="skeleton sk-line medium"></span>
            <span class="skeleton sk-line short"></span>
          </div>
        {/each}
      </div>
    {/if}
  </div>
</section>

<style>
  .skeleton-region {
    width: 100%;
  }

  .skeleton-layout {
    display: grid;
    gap: var(--space-stack);
  }

  .sk-panel {
    display: grid;
    gap: 0.65rem;
    padding: var(--space-card);
    border: 1px solid var(--line);
    border-radius: var(--radius-control);
    background: linear-gradient(180deg, var(--surface-top), var(--surface) 72%);
    box-shadow: var(--card-highlight), var(--tc-shadow-1);
  }

  .sk-kpis {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 0.55rem;
  }

  .sk-kpi {
    align-content: start;
    min-height: 6.2rem;
  }

  .sk-split {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-stack);
  }

  .sk-line {
    display: block;
    width: 100%;
    height: 0.75rem;
    border-radius: 999px;
  }

  .sk-line.short {
    width: 34%;
  }

  .sk-line.medium {
    width: 62%;
  }

  .sk-line.title {
    width: 46%;
    height: 1.1rem;
  }

  .sk-line.value {
    width: 52%;
    height: 1.7rem;
  }

  .sk-line.pill {
    width: 40%;
    height: 1.25rem;
  }

  .sk-dot {
    width: 2.75rem;
    height: 2.75rem;
    flex: 0 0 2.75rem;
    border-radius: var(--radius-chip);
  }

  .sk-row {
    display: flex;
    align-items: center;
    gap: 0.7rem;
    padding-top: 0.55rem;
    border-top: 1px solid var(--line);
  }

  .sk-row .sk-line:first-of-type {
    flex: 1;
  }

  .sk-row .sk-line.short {
    width: 22%;
    flex: 0 0 22%;
  }

  .sk-chart {
    display: block;
    width: 100%;
    height: 10.5rem;
    border-radius: var(--radius-control);
  }

  .sk-media {
    padding: 0;
    overflow: hidden;
  }

  .sk-photo {
    display: block;
    width: 100%;
    aspect-ratio: 4 / 3;
    border-radius: 0;
  }

  .sk-body {
    display: grid;
    gap: 0.65rem;
    padding: var(--space-card);
  }

  .sk-actions {
    display: flex;
    gap: 0.5rem;
    margin-top: 0.35rem;
  }

  .sk-button {
    display: block;
    flex: 1;
    height: 2.75rem;
    border-radius: 999px;
  }

  .sk-settings {
    display: grid;
    grid-template-columns: minmax(0, 1fr);
    gap: var(--space-stack);
  }

  @media (min-width: 768px) {
    .sk-kpis {
      grid-template-columns: repeat(4, minmax(0, 1fr));
    }

    .sk-kpis.three {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }

    .sk-settings {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
  }

  @media (min-width: 992px) {
    .sk-split {
      grid-template-columns: minmax(0, 1.6fr) minmax(0, 1fr);
    }
  }

  @media (max-width: 400px) {
    .sk-kpis.three {
      grid-template-columns: repeat(3, minmax(0, 1fr));
    }
  }
</style>
