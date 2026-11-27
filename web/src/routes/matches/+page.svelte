<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { goto } from '$app/navigation';
  import { apiJSON, relativeDate, type Match } from '#lib/api';
  import { setMatches } from '#lib/matchState';
  import SafetyDialog from '#lib/SafetyDialog.svelte';
  import { swipeRow } from '#lib/swipe';

  let matches: Match[] = [];
  let loading = true;
  let error = '';
  let safetyTarget: Match | null = null;
  let safety: { openReport: () => Promise<void>; openBlock: () => Promise<void> };

  $: total = matches.length;
  $: unrevealed = matches.filter((match) => !match.revealed).length;
  $: thisMonth = matches.filter((match) => {
    const date = new Date(match.matched_at);
    const now = new Date();
    return date.getFullYear() === now.getFullYear() && date.getMonth() === now.getMonth();
  }).length;

  onMount(async () => {
    try {
      matches = (await apiJSON<{ matches: Match[] }>('/api/v1/matches')).matches;
      setMatches(matches);
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'Matches are unavailable.';
    } finally {
      loading = false;
    }
  });

  function removeMatch(id: string) {
    matches = matches.filter((match) => match.match_id !== id);
    setMatches(matches);
  }

  async function openSafety(match: Match, mode: 'report' | 'block') {
    safetyTarget = match;
    await tick();
    if (mode === 'block') {
      await safety.openBlock();
    } else {
      await safety.openReport();
    }
  }
</script>

<svelte:head>
  <title>Matches · Tennis Connect</title>
</svelte:head>

<section class="page matches-page stack">
  <header class="page-head">
    <div>
      <p class="kicker">Mutual</p>
      <h1>Matches</h1>
    </div>
    <a class="btn btn-primary btn-sm" href="/discover"><i class="bi bi-compass" aria-hidden="true"></i> Discover</a>
  </header>

  {#if loading}
    <p role="status">Loading matches…</p>
  {:else if error}
    <p class="alert-box" role="alert">{error}</p>
  {:else}
    <section class="kpi-strip three" aria-label="Match metrics">
      <article class="kpi">
        <h2 class="kpi-label">Total matches</h2>
        <p class="kpi-value">{total}</p>
        <p class="kpi-delta">All time</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">New matches</h2>
        <p class="kpi-value">{unrevealed}</p>
        <p class="kpi-delta">{unrevealed ? 'Waiting to reveal' : 'All revealed'}</p>
      </article>
      <article class="kpi">
        <h2 class="kpi-label">This month</h2>
        <p class="kpi-value">{thisMonth}</p>
        <p class="kpi-delta">New connections</p>
      </article>
    </section>

    {#if !matches.length}
      <div class="sheet-card stack empty-state">
        <h2>No matches yet</h2>
        <p class="muted">A match appears only when you and another player both choose “I'd play”.</p>
        <a class="btn btn-primary align-self-start" href="/discover">Go to Discover</a>
      </div>
    {:else}
      <section class="sheet-card" aria-labelledby="match-list-title">
        <div class="spread list-head">
          <h2 id="match-list-title" class="mb-0">Your matches</h2>
          <span class="muted">Newest first</span>
        </div>
        <div class="data-table-wrap">
          <table class="data-table matches-table">
            <caption class="visually-hidden">Your mutual matches, newest first.</caption>
            <thead>
              <tr>
                <th scope="col">Player</th>
                <th scope="col">Matched</th>
                <th scope="col">Status</th>
                <th scope="col">Instagram</th>
                <th scope="col"><span class="visually-hidden">Actions</span></th>
              </tr>
            </thead>
            <tbody>
              {#each matches as match (match.match_id)}
                <tr
                  class="swipe-row"
                  class:new-match={!match.revealed}
                  use:swipeRow={{ onOpen: () => goto(`/matches/${match.match_id}`) }}
                >
                  <td class="swipe-cue" aria-hidden="true">
                    <span class="swipe-cue-label"><i class="bi bi-arrow-up-right" aria-hidden="true"></i> Open match</span>
                  </td>
                  <td data-label="Player">
                    <a class="match-person" href={`/matches/${match.match_id}`}>
                      {#if match.photo_urls.length}
                        <img class="thumb" src={match.photo_urls[0]} alt="" />
                      {:else}
                        <span class="thumb placeholder" aria-hidden="true"></span>
                      {/if}
                      <span class="match-copy">
                        <strong>{match.display_name}</strong>
                      </span>
                    </a>
                  </td>
                  <td data-label="Matched"><span class="matched-line"><span class="mobile-only">Matched</span>{' '}<span class="matched-value">{relativeDate(match.matched_at)}</span></span></td>
                  <td data-label="Status">
                    {#if !match.revealed}
                      <span class="chip optic">New match</span>
                    {:else}
                      <span class="chip success">Revealed</span>
                    {/if}
                  </td>
                  <td data-label="Instagram">
                    <span class="chip" class:info={match.has_instagram}>
                      <i class="bi bi-instagram me-1" aria-hidden="true"></i>{match.has_instagram ? 'Instagram available' : 'No Instagram'}
                    </span>
                  </td>
                  <td data-label="Actions" class="match-actions">
                    <a class="row-action" href={`/matches/${match.match_id}`} aria-label={`Open match with ${match.display_name}`}>
                      <i class="bi bi-arrow-up-right" aria-hidden="true"></i>
                    </a>
                    <button
                      class="icon-button dark"
                      type="button"
                      onclick={() => openSafety(match, 'report')}
                      aria-label={`Safety options for ${match.display_name}`}
                    >
                      <i class="bi bi-flag" aria-hidden="true"></i>
                    </button>
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </section>
    {/if}
  {/if}

  <SafetyDialog
    bind:this={safety}
    targetId={safetyTarget?.player_id ?? ''}
    targetName={safetyTarget?.display_name ?? 'this player'}
    onBlocked={() => {
      if (safetyTarget) removeMatch(safetyTarget.match_id);
    }}
    onReported={() => {
      if (safetyTarget) removeMatch(safetyTarget.match_id);
    }}
  />
</section>

<style>
  .matches-page {
    width: min(100% - (2 * var(--space-page-x)), 46rem);
  }

  .list-head {
    align-items: center;
    margin-bottom: 0.5rem;
  }

  @media (min-width: 768px) {
    .matches-table {
      min-width: 38rem;
    }
  }

  @media (min-width: 992px) {
    .matches-page {
      width: min(100% - 2rem, 64rem);
    }
  }

  .match-actions {
    display: flex;
    align-items: center;
    gap: 0.15rem;
  }

  .match-actions .row-action {
    color: var(--court);
  }

  .swipe-cue {
    display: none;
  }

  .match-person {
    display: flex;
    align-items: center;
    gap: 0.7rem;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
    transition: transform 160ms ease;
  }

  .match-person:hover {
    transform: translateX(2px);
  }

  .match-person:hover strong {
    text-decoration: underline;
    text-underline-offset: 0.18em;
  }

  .thumb {
    width: 2.75rem;
    height: 2.75rem;
    flex: 0 0 2.75rem;
    border: 1px solid var(--line);
    border-radius: var(--radius-chip);
    object-fit: cover;
    background: var(--media-bg);
    box-shadow: inset 0 1px 0 var(--highlight), var(--tc-shadow-1);
  }

  .placeholder {
    display: block;
    background: var(--media-bg);
  }

  .match-copy {
    display: grid;
    min-width: 0;
  }

  .match-copy strong {
    font-size: 0.95rem;
  }

  .mobile-only {
    display: none;
  }

  @media (max-width: 767px) {
    .mobile-only {
      display: inline;
    }

    .matches-table tbody tr {
      padding-right: 6.8rem;
      touch-action: pan-y;
    }

    .matches-table tbody td:not(.swipe-cue):not(:last-child) {
      position: relative;
      z-index: 2;
      transform: translateX(var(--swipe-offset, 0px));
      transition: transform 180ms ease;
    }

    .matches-table tbody tr:global(.is-swiping) td:not(.swipe-cue):not(:last-child) {
      transition: none;
    }

    .swipe-cue {
      position: absolute;
      inset: 0;
      z-index: 0;
      display: grid;
      place-items: center;
      margin: 0;
      padding: 0;
      pointer-events: none;
    }

    .swipe-cue::before {
      display: none;
    }

    .swipe-cue-label {
      display: inline-flex;
      align-items: center;
      gap: 0.35rem;
      padding: 0.35rem 0.8rem;
      border-radius: 999px;
      background: var(--chip-bg);
      color: var(--court);
      font-size: 0.78rem;
      font-weight: 800;
      letter-spacing: 0.02em;
      opacity: 0;
      transform: scale(0.94);
      transition: opacity 140ms ease, transform 140ms ease, box-shadow 140ms ease;
    }

    .matches-table tbody tr:global(.is-revealed) .swipe-cue-label,
    .matches-table tbody tr:global(.is-armed) .swipe-cue-label {
      opacity: 1;
      transform: none;
    }

    .matches-table tbody tr:global(.is-armed) .swipe-cue-label {
      background: linear-gradient(180deg, #efff9e, var(--ball));
      color: #0a1714;
      box-shadow: 0 0 16px var(--ball-glow);
    }

    .matches-table td[data-label='Player'] {
      order: 1;
      flex: 1 1 7rem;
    }

    .matches-table td[data-label='Status'] {
      order: 2;
      margin-left: auto;
    }

    .matches-table tbody tr::after {
      content: '';
      order: 3;
      flex-basis: 100%;
      height: 0;
    }

    .matches-table td[data-label='Matched'] {
      order: 4;
    }

    .matches-table td[data-label='Instagram'] {
      order: 5;
    }

    .matches-table td[data-label='Matched'],
    .matches-table td[data-label='Instagram'] {
      color: var(--muted);
      font-size: 0.8rem;
      line-height: 1.3;
    }

    .matches-table td[data-label='Matched']::after {
      content: '·';
      margin-left: 0.35rem;
    }

    .matches-table .chip {
      min-height: 1.5rem;
      padding: 0.1rem 0.5rem;
      font-size: 0.68rem;
    }
  }

  @media (prefers-reduced-motion: reduce) {
    .matches-table tbody tr,
    .matches-table tbody td:not(.swipe-cue):not(:last-child),
    .swipe-cue-label {
      transition: none !important;
    }
  }
</style>
