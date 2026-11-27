<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { apiJSON, relativeDate, type Match } from '#lib/api';
  import { setMatches } from '#lib/matchState';
  import SafetyDialog from '#lib/SafetyDialog.svelte';

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
                <tr class:new-match={!match.revealed}>
                  <td data-label="Player">
                    <a class="match-person" href={`/matches/${match.match_id}`}>
                      {#if match.photo_urls.length}
                        <img class="thumb" src={match.photo_urls[0]} alt="" />
                      {:else}
                        <span class="thumb placeholder" aria-hidden="true"></span>
                      {/if}
                      <span class="match-copy">
                        <strong>{match.display_name}</strong>
                        <span class="muted">Matched {relativeDate(match.matched_at)}</span>
                      </span>
                    </a>
                  </td>
                  <td data-label="Matched">{relativeDate(match.matched_at)}</td>
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
                  <td data-label="Actions">
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
    width: 2.5rem;
    height: 2.5rem;
    flex: 0 0 2.5rem;
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

  .match-copy .muted {
    font-size: 0.78rem;
  }
</style>
