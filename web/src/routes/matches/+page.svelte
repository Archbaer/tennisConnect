<script lang="ts">
  import { onMount, tick } from 'svelte';
  import { apiJSON, relativeDate, type Match } from '#lib/api';
  import SafetyDialog from '#lib/SafetyDialog.svelte';

  let matches: Match[] = [];
  let loading = true;
  let error = '';
  let safetyTarget: Match | null = null;
  let safety: { openReport: () => Promise<void>; openBlock: () => Promise<void> };

  onMount(async () => {
    try {
      matches = (await apiJSON<{ matches: Match[] }>('/api/v1/matches')).matches;
    } catch (problem) {
      error = problem instanceof Error ? problem.message : 'Matches are unavailable.';
    } finally {
      loading = false;
    }
  });

  function removeMatch(id: string) {
    matches = matches.filter((match) => match.match_id !== id);
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
  <div>
    <p class="score mb-1"><span class="tennis-ball me-2" aria-hidden="true"></span>Mutual</p>
    <h1>Matches</h1>
  </div>

  {#if loading}
    <p role="status">Loading matches…</p>
  {:else if error}
    <p class="alert-box" role="alert">{error}</p>
  {:else if !matches.length}
    <div class="surface stack">
      <h2>No matches yet</h2>
      <p class="muted">A match appears only when you and another player both choose “I'd play”.</p>
      <a class="btn btn-primary align-self-start" href="/discover">Go to Discover</a>
    </div>
  {:else}
    <ul class="match-list">
      {#each matches as match (match.match_id)}
        <li class="match-row">
          <a href={`/matches/${match.match_id}`} class="match-link">
            {#if match.photo_urls.length}
              <img class="photo thumb" src={match.photo_urls[0]} alt="" />
            {:else}
              <div class="photo thumb placeholder" aria-hidden="true"></div>
            {/if}
            <span class="match-copy">
              <strong>{match.display_name}</strong>
              <span class="muted">Matched {relativeDate(match.matched_at)}</span>
              <span class="chip" class:optic={match.has_instagram}>
                <i class="bi bi-instagram me-1" aria-hidden="true"></i>{match.has_instagram ? 'Instagram available' : 'No Instagram'}
              </span>
            </span>
          </a>
          <button
            class="icon-button dark"
            type="button"
            onclick={() => openSafety(match, 'report')}
            aria-label={`Safety options for ${match.display_name}`}
          >
            <i class="bi bi-flag" aria-hidden="true"></i>
          </button>
        </li>
      {/each}
    </ul>
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
    width: min(100% - 1.5rem, 42rem);
  }

  .match-list {
    list-style: none;
    margin: 0;
    padding: 0;
    display: grid;
    gap: 0.75rem;
  }

  .match-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    border: 1px solid var(--line);
    border-radius: var(--radius);
    background: var(--surface);
    box-shadow: var(--shadow-small);
    padding: 0.6rem;
  }

  .match-link {
    display: flex;
    flex: 1;
    align-items: center;
    gap: 1rem;
    min-width: 0;
    color: var(--ink);
    text-decoration: none;
  }

  .thumb {
    width: 5rem;
    flex: 0 0 5rem;
    border-radius: var(--radius-control);
  }

  .placeholder {
    background: #e5eae7;
  }

  .match-copy {
    display: grid;
    gap: 0.25rem;
    min-width: 0;
  }

  .match-copy strong {
    font-size: 1.05rem;
  }

  .match-copy .chip {
    justify-self: start;
  }
</style>
